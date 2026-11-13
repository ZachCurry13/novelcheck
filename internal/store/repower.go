package store

// Electricity for calls already made: each call's seconds are kept, so when
// a machine's watts or the electricity price are set (or changed), the cost
// of the calls its models made is worked out again, back to the first one.

import "strings"

// costRule is how one model's calls are priced.
type costRule struct{ priceIn, priceOut, watts float64 }

// localRules maps each model of an AI that runs on a machine of the
// family's own (no token prices) to its pricing: the Deep Scan machine,
// then the backup, then the main AI; a model no AI lists any more that looks
// like a local one ("name:tag") counts as the main AI's.
func (s *Store) localRules() (rules map[string]costRule, mainRule costRule) {
	rules = map[string]costRule{}
	add := func(ai AIConfig) {
		if ai.PriceIn > 0 || ai.PriceOut > 0 {
			return // a paid service: its token prices are what it cost
		}
		for _, m := range ai.Models {
			if _, ok := rules[m]; !ok {
				rules[m] = costRule{watts: ai.Watts}
			}
		}
	}
	if deep, ok := s.DeepAI(); ok {
		add(deep)
	}
	ais := s.AIConfigs()
	for i := len(ais) - 1; i >= 0; i-- {
		add(ais[i])
	}
	mainRule = costRule{watts: ais[0].Watts}
	if ais[0].PriceIn > 0 || ais[0].PriceOut > 0 {
		mainRule = costRule{watts: -1} // the main AI is paid: unknown models aren't guessed
	}
	if v := s.Setting(KeyVisionModel); v != "" {
		if _, ok := rules[v]; !ok && mainRule.watts >= 0 {
			rules[v] = mainRule // photo checks run on the main machine
		}
	}
	return rules, mainRule
}

// RecountElectricity prices every call of a local model again with today's
// watts and electricity price. A call from before call times were kept is
// given its model's average time per token. It returns the calls repriced.
func (s *Store) RecountElectricity() (int, error) {
	rules, mainRule := s.localRules()
	price := s.SettingFloat(KeyPowerPrice)
	var models []string
	if err := s.DB.Select(&models, `SELECT DISTINCT model FROM token_usage`); err != nil {
		return 0, err
	}
	n := 0
	for _, m := range models {
		r, ok := rules[m]
		if !ok && strings.Contains(m, ":") && mainRule.watts >= 0 {
			r, ok = mainRule, true
		}
		if !ok {
			continue
		}
		perHour := 0.0
		if r.watts > 0 && price > 0 {
			perHour = r.watts / 1000 * price // dollars per hour of work
		}
		var perToken float64 // seconds per token, for calls with no time kept
		_ = s.DB.Get(&perToken, `SELECT COALESCE(SUM(seconds) / NULLIF(SUM(prompt_tokens + completion_tokens), 0), 0)
			FROM token_usage WHERE model = ? AND seconds > 0`, m)
		res, err := s.DB.Exec(`UPDATE token_usage SET cost =
			(CASE WHEN seconds > 0 THEN seconds ELSE (prompt_tokens + completion_tokens) * ? END) / 3600.0 * ?
			WHERE model = ?`, perToken, perHour, m)
		if err != nil {
			return n, err
		}
		k, _ := res.RowsAffected()
		n += int(k)
	}
	return n, nil
}

// ElectricityUSD is what the calls of local models cost in electricity.
func (s *Store) ElectricityUSD() float64 {
	rules, mainRule := s.localRules()
	var rows []struct {
		Model string  `db:"model"`
		Cost  float64 `db:"cost"`
	}
	_ = s.DB.Select(&rows, `SELECT model, COALESCE(SUM(cost), 0) AS cost FROM token_usage GROUP BY model`)
	total := 0.0
	for _, r := range rows {
		if _, ok := rules[r.Model]; ok || (strings.Contains(r.Model, ":") && mainRule.watts >= 0) {
			total += r.Cost
		}
	}
	return total
}
