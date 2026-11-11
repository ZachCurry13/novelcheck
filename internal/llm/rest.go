package llm

import (
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// A service that says its quota is used up ("requests per day limit exceeded… please retry in 4h28m") will
// say so to every request until then. Asking again only wastes time and fills the logs with the same long
// message, so the model is left alone until the time it named, and callers get one short sentence instead.

var resting sync.Map // base URL + "|" + model -> time.Time it may be asked again

var retryIn = regexp.MustCompile(`(?i)retry in ((?:\d+h)?(?:\d+m)?(?:[\d.]+s)?)`)

// quotaRest is how long to leave a model alone after err, or 0 when err is not a used-up quota.
func quotaRest(err error) time.Duration {
	if err == nil {
		return 0
	}
	msg := err.Error()
	low := strings.ToLower(msg)
	if !strings.Contains(low, "quota") && !strings.Contains(low, "per day") && !strings.Contains(low, "daily limit") {
		return 0
	}
	if m := retryIn.FindStringSubmatch(msg); m != nil {
		if d, perr := time.ParseDuration(m[1]); perr == nil && d > 2*time.Minute {
			return min(d, 24*time.Hour)
		}
		return 0 // a short wait: the ordinary "busy, try again" handling covers it
	}
	if strings.Contains(low, "per day") || strings.Contains(low, "daily") {
		return time.Hour
	}
	return 0
}

// restingErr is the short sentence for a model that is being left alone.
func restingErr(model string, until time.Time) error {
	left := time.Until(until).Round(time.Minute)
	h, m := int(left.Hours()), int(left.Minutes())%60
	wait := fmt.Sprintf("%d min", max(m, 1))
	if h > 0 {
		wait = fmt.Sprintf("%d h %d min", h, m)
	}
	return fmt.Errorf("%s has used up its free requests for today (the service's own daily limit); NovelCheck will ask it again in %s. A paid plan with that service, or another backup model, lifts the limit", model, wait)
}

// rested returns an error at once when the model is being left alone.
func rested(baseURL, model string) error {
	v, ok := resting.Load(baseURL + "|" + model)
	if !ok {
		return nil
	}
	until := v.(time.Time)
	if time.Now().After(until) {
		resting.Delete(baseURL + "|" + model)
		return nil
	}
	return restingErr(model, until)
}

// noteQuota starts the rest when err says the quota is used up, and returns the short sentence for it.
func noteQuota(baseURL, model string, err error) error {
	d := quotaRest(err)
	if d == 0 {
		return err
	}
	until := time.Now().Add(d)
	resting.Store(baseURL+"|"+model, until)
	return restingErr(model, until)
}

// Resting lists the models being left alone and until when (for the Integrations page).
func Resting() map[string]time.Time {
	out := map[string]time.Time{}
	resting.Range(func(k, v any) bool {
		if until := v.(time.Time); time.Now().Before(until) {
			_, model, _ := strings.Cut(k.(string), "|")
			out[model] = until
		}
		return true
	})
	return out
}
