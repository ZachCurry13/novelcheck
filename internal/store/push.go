package store

// KeyPushPrivateKey holds the server's VAPID signing key (PKCS#8, base64).
// It is a secret: masked in diagnostics and never sent to the browser.
const KeyPushPrivateKey = "push_vapid_private_key"

func init() { SecretKeys[KeyPushPrivateKey] = true }

// PushSub is one phone or browser that turned on notifications.
type PushSub struct {
	ID        int64  `db:"id" json:"id"`
	UserID    int64  `db:"user_id" json:"-"`
	Endpoint  string `db:"endpoint" json:"-"`
	P256dh    string `db:"p256dh" json:"-"`
	Auth      string `db:"auth" json:"-"`
	Scope     string `db:"scope" json:"scope"`
	Device    string `db:"device" json:"device"`
	CreatedAt string `db:"created_at" json:"created_at"`
}

const pushCols = `s.id, s.user_id, s.endpoint, s.p256dh, s.auth, s.scope, s.device, s.created_at`

// SavePushSub adds a device, or updates it if the endpoint is already known
// (it then belongs to whoever registered it last).
func (s *Store) SavePushSub(p PushSub) error {
	_, err := s.DB.Exec(`INSERT INTO push_subscriptions (user_id, endpoint, p256dh, auth, scope, device)
		VALUES (?, ?, ?, ?, ?, ?) ON CONFLICT(endpoint) DO UPDATE SET user_id = excluded.user_id,
		p256dh = excluded.p256dh, auth = excluded.auth, scope = excluded.scope, device = excluded.device`,
		p.UserID, p.Endpoint, p.P256dh, p.Auth, p.Scope, p.Device)
	return err
}

// DeletePushSub removes one of the user's devices by endpoint.
func (s *Store) DeletePushSub(userID int64, endpoint string) error {
	_, err := s.DB.Exec(`DELETE FROM push_subscriptions WHERE user_id = ? AND endpoint = ?`, userID, endpoint)
	return err
}

// DropPushEndpoint forgets a device the push service says is gone.
func (s *Store) DropPushEndpoint(endpoint string) {
	_, _ = s.DB.Exec(`DELETE FROM push_subscriptions WHERE endpoint = ?`, endpoint)
}

// UserPushSubs lists one user's devices.
func (s *Store) UserPushSubs(userID int64) ([]PushSub, error) {
	var out []PushSub
	err := s.DB.Select(&out, `SELECT `+pushCols+` FROM push_subscriptions s WHERE s.user_id = ? ORDER BY s.id`, userID)
	return out, err
}

// ManagerPushSubs lists admins' and editors' devices that want a notice of
// this level: "all" devices get everything, "problems" only warnings/errors.
func (s *Store) ManagerPushSubs(level string) ([]PushSub, error) {
	var out []PushSub
	err := s.DB.Select(&out, `SELECT `+pushCols+` FROM push_subscriptions s JOIN users u ON u.id = s.user_id
		WHERE u.role IN ('admin', 'editor') AND (s.scope = 'all' OR ? IN ('warning', 'error')) ORDER BY s.id`, level)
	return out, err
}
