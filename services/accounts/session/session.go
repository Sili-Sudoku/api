// Package session makes session tokens and the cookie that carries them.
package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"time"
)

const (
	CookieName = "session"
	TTL        = 30 * 24 * time.Hour
)

// New returns a random token for the cookie and its hash for the database. Only the hash is stored.
func New() (token string, hash [32]byte, err error) {
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", hash, err
	}

	token = base64.RawURLEncoding.EncodeToString(raw)

	return token, Hash(token), nil
}

func Hash(token string) [32]byte { return sha256.Sum256([]byte(token)) }

// Cookie is the cookie of a session that ends at expires. insecure drops the Secure flag, for local http only.
func Cookie(token string, expires time.Time, insecure bool) *http.Cookie {
	return &http.Cookie{Name: CookieName, Value: token, Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: !insecure, Expires: expires}
}

// ClearCookie removes the cookie from the browser.
func ClearCookie(insecure bool) *http.Cookie {
	return &http.Cookie{Name: CookieName, Value: "", Path: "/", HttpOnly: true, SameSite: http.SameSiteLaxMode,
		Secure: !insecure, MaxAge: -1}
}
