// Package cursor encodes a keyset position as an opaque URL-safe string.
package cursor

import (
	"encoding/base64"
	"encoding/json"
)

func Encode(c any) string {
	b, _ := json.Marshal(c)
	return base64.RawURLEncoding.EncodeToString(b)
}

func Decode[T any](s string) (*T, bool) {
	b, err := base64.RawURLEncoding.DecodeString(s)
	var c T
	if err != nil || json.Unmarshal(b, &c) != nil {
		return nil, false
	}

	return &c, true
}
