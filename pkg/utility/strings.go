package utility

import (
	"regexp"
	"strings"
	"time"
)

// URLFragment converts a string into a URL-safe fragment.
//
// If no fragment can be determined, one is generated from the current time.
func URLFragment(v string) string {
	fragment := strings.ToLower(v)
	fragment = regexp.MustCompile(`[^A-Za-z0-9]+`).ReplaceAllString(fragment, "-")
	fragment = strings.ReplaceAll(fragment, "--", "-")
	fragment = strings.Trim(fragment, "-")
	if fragment == "" {
		fragment = time.Now().Format("20060102150405")
	}
	return fragment
}
