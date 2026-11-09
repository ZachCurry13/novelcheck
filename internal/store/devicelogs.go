package store

import "strings"

// DeviceLog is a log the KOReader plugin sent.
type DeviceLog struct {
	ID        int64  `db:"id" json:"id"`
	Username  string `db:"username" json:"username"`
	Device    string `db:"device" json:"device"`
	Version   string `db:"version" json:"version"`
	Size      int    `db:"size" json:"size"`
	CreatedAt string `db:"created_at" json:"created_at"`
}

const keepLogsPerDevice = 8

// SaveDeviceLog keeps a log from a device (the person's sync code is blanked
// out of it) and drops that device's older ones beyond the last few.
func (s *Store) SaveDeviceLog(u *User, device, version, body string) error {
	device, version = strings.TrimSpace(device), strings.TrimSpace(version)
	if len(device) > 80 {
		device = device[:80]
	}
	if len(version) > 20 {
		version = version[:20]
	}
	var code string
	if s.DB.Get(&code, `SELECT kosync_code FROM users WHERE id = ?`, u.ID) == nil && len(code) >= 4 {
		body = strings.ReplaceAll(body, code, "(sync code)")
	}
	if _, err := s.DB.Exec(`INSERT INTO device_logs (user_id, username, device, version, body) VALUES (?, ?, ?, ?, ?)`,
		u.ID, u.Username, device, version, body); err != nil {
		return err
	}
	_, err := s.DB.Exec(`DELETE FROM device_logs WHERE user_id = ? AND device = ? AND id NOT IN
		(SELECT id FROM device_logs WHERE user_id = ? AND device = ? ORDER BY id DESC LIMIT ?)`,
		u.ID, device, u.ID, device, keepLogsPerDevice)
	return err
}

// DeviceLogs lists the logs, newest first.
func (s *Store) DeviceLogs() ([]DeviceLog, error) {
	out := []DeviceLog{}
	err := s.DB.Select(&out, `SELECT id, username, device, version, LENGTH(body) AS size, created_at FROM device_logs ORDER BY id DESC LIMIT 100`)
	return out, err
}

// DeviceLogText is one log's text.
func (s *Store) DeviceLogText(id int64) (string, error) {
	var t string
	err := s.DB.Get(&t, `SELECT body FROM device_logs WHERE id = ?`, id)
	return t, err
}

func (s *Store) DeleteDeviceLog(id int64) error {
	_, err := s.DB.Exec(`DELETE FROM device_logs WHERE id = ?`, id)
	return err
}
