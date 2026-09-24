// Package push sends Web Push notifications (RFC 8030) to phones and
// browsers. Payloads are encrypted per RFC 8291 (aes128gcm) and requests are
// signed with VAPID (RFC 8292), using only the standard library.
package push

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// Subscription is what a browser's PushManager hands out: where to send,
// and the keys to encrypt for (base64url, as the browser gives them).
type Subscription struct {
	Endpoint string
	P256dh   string // the browser's P-256 public key (65 bytes, uncompressed)
	Auth     string // 16-byte authentication secret
}

// recordSize is the aes128gcm record size; one record carries the whole
// message, so payloads must stay under recordSize-17 bytes.
const recordSize = 4096

// MaxPayload is the largest plaintext that fits in one record.
const MaxPayload = recordSize - 16 - 1

// b64 decodes base64url with or without padding (browsers vary).
func b64(s string) ([]byte, error) {
	if b, err := base64.RawURLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.URLEncoding.DecodeString(s)
}

// ValidKeys reports whether a browser's subscription keys look right: a
// 65-byte uncompressed P-256 point and a 16-byte auth secret.
func ValidKeys(p256dh, auth string) bool {
	pub, err := b64(p256dh)
	if err != nil {
		return false
	}
	if _, err := ecdh.P256().NewPublicKey(pub); err != nil {
		return false
	}
	a, err := b64(auth)
	return err == nil && len(a) == 16
}

// encrypt builds an aes128gcm body for sub (RFC 8291 §3-4). asKey and salt
// are random per message; tests pass fixed ones.
func encrypt(sub Subscription, plaintext []byte, asKey *ecdh.PrivateKey, salt []byte) ([]byte, error) {
	if len(plaintext) > MaxPayload {
		return nil, fmt.Errorf("push payload too large (%d bytes)", len(plaintext))
	}
	uaBytes, err := b64(sub.P256dh)
	if err != nil {
		return nil, fmt.Errorf("bad p256dh key: %w", err)
	}
	authSecret, err := b64(sub.Auth)
	if err != nil || len(authSecret) != 16 {
		return nil, errors.New("bad auth secret")
	}
	uaPub, err := ecdh.P256().NewPublicKey(uaBytes)
	if err != nil {
		return nil, fmt.Errorf("bad p256dh key: %w", err)
	}
	shared, err := asKey.ECDH(uaPub)
	if err != nil {
		return nil, err
	}
	asPub := asKey.PublicKey().Bytes()
	ikm, err := hkdf.Key(sha256.New, shared, authSecret, "WebPush: info\x00"+string(uaBytes)+string(asPub), 32)
	if err != nil {
		return nil, err
	}
	cek, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	if err != nil {
		return nil, err
	}
	nonce, err := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(cek)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	// Header: salt, record size, key id length, and our public key as key id.
	var out bytes.Buffer
	out.Write(salt)
	_ = binary.Write(&out, binary.BigEndian, uint32(recordSize))
	out.WriteByte(byte(len(asPub)))
	out.Write(asPub)
	padded := append(append([]byte{}, plaintext...), 0x02) // last-record delimiter
	return gcm.Seal(out.Bytes(), nonce, padded, nil), nil
}

// vapidJWT signs the claim that we (subject) may push to aud until exp.
func vapidJWT(key *ecdsa.PrivateKey, aud, subject string, exp time.Time) (string, error) {
	enc := base64.RawURLEncoding
	claims, _ := json.Marshal(map[string]any{"aud": aud, "exp": exp.Unix(), "sub": subject})
	unsigned := enc.EncodeToString([]byte(`{"typ":"JWT","alg":"ES256"}`)) + "." + enc.EncodeToString(claims)
	digest := sha256.Sum256([]byte(unsigned))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		return "", err
	}
	sig := make([]byte, 64) // ES256 is r||s, 32 bytes each
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return unsigned + "." + enc.EncodeToString(sig), nil
}

// Sender signs and delivers messages with one VAPID key.
type Sender struct {
	Key     *ecdsa.PrivateKey
	Subject string // contact for push services: an https:// URL or mailto:
	Client  *http.Client
}

// ErrGone means the browser unsubscribed (or the subscription expired):
// delete it and stop sending to it.
var ErrGone = errors.New("push subscription is gone")

// Send encrypts payload for sub and posts it to the push service. ttl is how
// long the service may hold it for an offline device.
func (s *Sender) Send(ctx context.Context, sub Subscription, payload []byte, ttl time.Duration) error {
	u, err := url.Parse(sub.Endpoint)
	if err != nil || u.Host == "" {
		return errors.New("bad push endpoint")
	}
	asKey, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return err
	}
	body, err := encrypt(sub, payload, asKey, salt)
	if err != nil {
		return err
	}
	jwt, err := vapidJWT(s.Key, u.Scheme+"://"+u.Host, s.Subject, time.Now().Add(12*time.Hour))
	if err != nil {
		return err
	}
	pub, err := s.Key.PublicKey.ECDH()
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, sub.Endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/octet-stream")
	req.Header.Set("Content-Encoding", "aes128gcm")
	req.Header.Set("TTL", strconv.Itoa(int(ttl.Seconds())))
	req.Header.Set("Urgency", "normal")
	req.Header.Set("Authorization", "vapid t="+jwt+", k="+base64.RawURLEncoding.EncodeToString(pub.Bytes()))
	client := s.Client
	if client == nil {
		client = &http.Client{Timeout: 15 * time.Second}
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	msg, _ := io.ReadAll(io.LimitReader(res.Body, 512))
	switch {
	case res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone:
		return ErrGone
	case res.StatusCode >= 300:
		return fmt.Errorf("push service answered %s: %s", res.Status, bytes.TrimSpace(msg))
	}
	return nil
}
