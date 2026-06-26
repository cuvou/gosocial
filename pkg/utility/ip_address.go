package utility

import (
	"net"
	"net/http"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
)

/*
IPAddress returns the best guess at the user's IP address, as a string for logging.
*/
func IPAddress(r *http.Request) string {

	// Configured for proxy IP addresses?
	if config.Current.UseXForwardedFor {

		// 1. Check X-Real-IP header
		if ip := strings.TrimSpace(r.Header.Get("X-Real-IP")); ip != "" {
			if parsedIP := net.ParseIP(ip); parsedIP != nil {
				return parsedIP.String()
			} else {
				log.Warn("utility.IPAddress: X-Real-IP '%s' did not parse!", ip)
			}
		}

		// 2. Check X-Forwarded-For header (may contain multiple comma-separated IPs)
		if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
			// Split and iterate to find the first valid IP
			parts := strings.Split(xff, ",")
			for _, part := range parts {
				ip := strings.TrimSpace(part)
				if parsedIP := net.ParseIP(ip); parsedIP != nil {
					return parsedIP.String()
				} else {
					log.Warn("utility.IPAddress: X-Forwarded-For '%s' did not parse!", ip)
				}
			}
		}

	}

	// 3. Fallback: use RemoteAddr
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// If splitting fails (rare), try parsing whole RemoteAddr
		if parsedIP := net.ParseIP(r.RemoteAddr); parsedIP != nil {
			return parsedIP.String()
		}
		return ""
	}

	return ip
}
