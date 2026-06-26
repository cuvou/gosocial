package middleware

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/utility"
)

// AgeGate: part of LoginRequired that verifies the user has a birthdate on file.
//
// Also drives the Security Checkup interstitial to remind the user to configure 2FA.
func AgeGate(user *models.User, w http.ResponseWriter, r *http.Request) (handled bool) {

	// NOTE: not on /htmx/ endpoints so they can be decorated by LoginRequired and not
	// redirect to other pages.
	if strings.HasPrefix(r.URL.Path, "/htmx/") {
		return false
	}

	// Whitelisted endpoints where we won't redirect them away
	var whitelistedPaths = []string{
		"/me",
		"/account",
		"/settings",
		"/messages",
		"/friends",
		"/u/",
		"/photo/upload",
		"/photo/certification",
		"/photo/private",
		"/photo/view",
		"/photo/media",
		"/comments",
		"/notes/me",
		"/users/blocked",
		"/users/block",
		"/users/muted",
		"/account/delete",
		"/v1/", // API endpoints like the Like buttons
	}
	for _, path := range whitelistedPaths {
		if strings.HasPrefix(r.URL.Path, path) {
			return
		}
	}

	// User has no age set? Redirect them to the age gate prompt.
	if user.Birthdate.IsZero() || utility.Age(user.Birthdate) < 18 {
		templates.Redirect(w, "/settings/age-gate")
		return true
	}

	// Security Checkup eligible? (Enable 2FA prompt).
	if currentUser, err := session.CurrentUser(r); err == nil {
		if currentUser.GetProfileField("security_checkup_eligible") != "" {
			templates.Redirect(w, "/settings/security-checkup?next="+url.QueryEscape(r.URL.String()))
			return true
		}
	}

	return
}
