// Package clientip finds the address of the caller, for rate limits.
package clientip

import (
	"net"
	"strings"
)

// Resolve takes the RemoteAddr and the X-Forwarded-For header of a request. Behind a trusted proxy the last entry of the
// header is the address that the proxy saw, so that is the one to trust.
func Resolve(remoteAddr, forwardedFor string, trustProxy bool) string {
	if trustProxy && forwardedFor != "" {
		parts := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(parts[len(parts)-1])
	}
	host, _, err := net.SplitHostPort(remoteAddr)
	if err != nil {
		return remoteAddr
	}

	return host
}
