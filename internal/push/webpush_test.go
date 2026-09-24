package push

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hkdf"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func mustB64(t *testing.T, s string) []byte {
	t.Helper()
	b, err := b64(strings.Join(strings.Fields(s), ""))
	if err != nil {
		t.Fatalf("decode %q: %v", s, err)
	}
	return b
}

// The worked example from RFC 8291 Section 5 / Appendix A.
func TestEncryptMatchesRFC8291(t *testing.T) {
	asKey, err := ecdh.P256().NewPrivateKey(mustB64(t, "yfWPiYE-n46HLnH0KqZOF1fJJU3MYrct3AELtAQ-oRw"))
	if err != nil {
		t.Fatal(err)
	}
	sub := Subscription{
		P256dh: "BCVxsr7N_eNgVRqvHtD0zTZsEc6-VV-JvLexhqUzORcxaOzi6-AYWXvTBHm4bjyPjs7Vd8pZGH6SRpkNtoIAiw4",
		Auth:   "BTBZMqHH6r4Tts7J_aSIgg",
	}
	got, err := encrypt(sub, []byte("When I grow up, I want to be a watermelon"), asKey, mustB64(t, "DGv6ra1nlYgDCS1FRnbzlw"))
	if err != nil {
		t.Fatal(err)
	}
	header := mustB64(t, `DGv6ra1nlYgDCS1FRnbzlwAAEABBBP4z 9KsN6nGRTbVYI_c7VJSPQTBtkgcy27ml
		mlMoZIIgDll6e3vCYLocInmYWAmS6Tlz AC8wEqKK6PBru3jl7A8`)
	ciphertext := mustB64(t, `8pfeW0KbunFT06SuDKoJH9Ql87S1QUrd irN6GcG7sFz1y1sqLgVi1VhjVkHsUoEs bI_0LpXMuGvnzQ`)
	if want := append(header, ciphertext...); !bytes.Equal(got, want) {
		t.Fatalf("encrypted body differs from RFC 8291:\n got %x\nwant %x", got, want)
	}
}

// decrypt is the browser's side (RFC 8291), to read what Send posted.
func decrypt(t *testing.T, ua *ecdh.PrivateKey, auth, body []byte) []byte {
	t.Helper()
	salt, keyLen := body[:16], int(body[20])
	asPub, ct := body[21:21+keyLen], body[21+keyLen:]
	pub, err := ecdh.P256().NewPublicKey(asPub)
	if err != nil {
		t.Fatal(err)
	}
	shared, _ := ua.ECDH(pub)
	ikm, _ := hkdf.Key(sha256.New, shared, auth, "WebPush: info\x00"+string(ua.PublicKey().Bytes())+string(asPub), 32)
	cek, _ := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: aes128gcm\x00", 16)
	nonce, _ := hkdf.Key(sha256.New, ikm, salt, "Content-Encoding: nonce\x00", 12)
	block, _ := aes.NewCipher(cek)
	gcm, _ := cipher.NewGCM(block)
	plain, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	return bytes.TrimSuffix(plain, []byte{0x02})
}

func TestSendSignsAndEncrypts(t *testing.T) {
	ua, _ := ecdh.P256().GenerateKey(rand.Reader)
	auth := make([]byte, 16)
	_, _ = rand.Read(auth)
	vapid, _ := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)

	var got *http.Request
	var body []byte
	status := http.StatusCreated
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got, body = r, must(io.ReadAll(r.Body))
		w.WriteHeader(status)
	}))
	defer srv.Close()
	sub := Subscription{Endpoint: srv.URL + "/push/abc", P256dh: base64.RawURLEncoding.EncodeToString(ua.PublicKey().Bytes()),
		Auth: base64.RawURLEncoding.EncodeToString(auth)}
	s := &Sender{Key: vapid, Subject: "https://example.org"}
	if err := s.Send(context.Background(), sub, []byte(`{"title":"Hi"}`), time.Hour); err != nil {
		t.Fatal(err)
	}
	if got.Header.Get("Content-Encoding") != "aes128gcm" || got.Header.Get("TTL") != "3600" {
		t.Fatalf("headers: %v", got.Header)
	}
	if plain := decrypt(t, ua, auth, body); string(plain) != `{"title":"Hi"}` {
		t.Fatalf("payload: %q", plain)
	}

	// Authorization: vapid t=<JWT>, k=<our public key>, and the JWT verifies.
	h := got.Header.Get("Authorization")
	parts := strings.SplitN(strings.TrimPrefix(h, "vapid t="), ", k=", 2)
	pub, _ := vapid.PublicKey.ECDH()
	if len(parts) != 2 || parts[1] != base64.RawURLEncoding.EncodeToString(pub.Bytes()) {
		t.Fatalf("authorization: %q", h)
	}
	seg := strings.Split(parts[0], ".")
	var claims struct {
		Aud string `json:"aud"`
		Exp int64  `json:"exp"`
		Sub string `json:"sub"`
	}
	_ = json.Unmarshal(mustB64(t, seg[1]), &claims)
	if claims.Aud != srv.URL || claims.Sub != "https://example.org" || claims.Exp < time.Now().Unix() {
		t.Fatalf("claims: %+v", claims)
	}
	sig := mustB64(t, seg[2])
	digest := sha256.Sum256([]byte(seg[0] + "." + seg[1]))
	if !ecdsa.Verify(&vapid.PublicKey, digest[:], new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])) {
		t.Fatal("VAPID signature does not verify")
	}

	// A browser that unsubscribed answers 410: the caller must forget it.
	status = http.StatusGone
	if err := s.Send(context.Background(), sub, []byte("x"), time.Hour); err != ErrGone {
		t.Fatalf("410 should be ErrGone, got %v", err)
	}
	if err := s.Send(context.Background(), sub, bytes.Repeat([]byte("x"), MaxPayload+1), time.Hour); err == nil {
		t.Fatal("oversized payload must be refused")
	}
}

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}
