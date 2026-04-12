package http

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
)

// GetClientIP returns the client's real IP address, respecting trusted proxies.
// It uses a secure backwards-walking algorithm on the X-Forwarded-For header.
func GetClientIP(r *http.Request, trustedProxies []netip.Prefix) string {
	remote := requestRemoteIP(r)
	remoteAddr, err := netip.ParseAddr(remote)
	if err != nil {
		return remote
	}

	// If no trusted proxies configured, use remote address directly
	if len(trustedProxies) == 0 {
		return remote
	}

	// If remote IP is NOT a trusted proxy, it is the client IP
	if !IsTrustedProxy(remoteAddr, trustedProxies) {
		return remote
	}

	// 1. Check CF-Connecting-IP (Cloudflare) - Highest priority if it exists
	if cfIP := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); cfIP != "" {
		if addr, err := netip.ParseAddr(cfIP); err == nil {
			return addr.String()
		}
	}

	// 2. Check X-Forwarded-For - Walk backwards securely
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		parts := strings.Split(xff, ",")
		var bestCandidate string
		// Walk backwards through the chain
		for i := len(parts) - 1; i >= 0; i-- {
			part := strings.TrimSpace(parts[i])
			addr, err := netip.ParseAddr(part)
			if err != nil {
				continue
			}

			if !IsTrustedProxy(addr, trustedProxies) {
				// Found the first untrusted IP in the chain - this is the client
				return addr.String()
			}
			// If it is a trusted proxy, it *could* be the client if the chain started there
			if bestCandidate == "" {
				bestCandidate = addr.String()
			}
		}

		if bestCandidate != "" {
			return bestCandidate
		}
	}

	// 3. Check X-Real-IP - Fallback for other proxies
	if xri := strings.TrimSpace(r.Header.Get("X-Real-IP")); xri != "" {
		if addr, err := netip.ParseAddr(xri); err == nil {
			return addr.String()
		}
	}

	// 4. Check X-Client-IP (Legacy fallback)
	if xci := strings.TrimSpace(r.Header.Get("X-Client-IP")); xci != "" {
		if addr, err := netip.ParseAddr(xci); err == nil {
			return addr.String()
		}
	}

	return remote
}

// IsTrustedProxy checks if an IP address matches any of the trusted proxy prefixes.
func IsTrustedProxy(addr netip.Addr, trusted []netip.Prefix) bool {
	for _, prefix := range trusted {
		if prefix.Contains(addr) {
			return true
		}
	}
	return false
}

func requestRemoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(strings.TrimSpace(r.RemoteAddr))
	if err == nil {
		return host
	}
	return strings.TrimSpace(r.RemoteAddr)
}
