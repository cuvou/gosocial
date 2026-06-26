package middleware

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/google/uuid"
)

// CSRFBypassPrefixes names URL routes that CSRF protection doesn't apply to, e.g. non-JSON API webhooks URLs.
var CSRFBypassPrefixes = []string{
	"/v1/billing/ccbill",
}

// CSRF middleware. Other places to look: pkg/session/session.go, pkg/templates/template_funcs.go
func CSRF(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get or create the cookie CSRF value.
		token := MakeCSRFCookie(r, w)
		ctx := context.WithValue(r.Context(), session.CSRFKey, token)

		// Store the request start time.
		ctx = context.WithValue(ctx, session.RequestTimeKey, time.Now())

		// If it's a JSON post, allow it thru.
		if r.Header.Get("Content-Type") == "application/json" {
			handler.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Bypass CSRF checks for this URL?
		for _, prefix := range CSRFBypassPrefixes {
			if strings.HasPrefix(r.URL.Path, prefix) {
				handler.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}

		// If we are running a POST request, validate the CSRF form value.
		if r.Method != http.MethodGet {
			r.ParseMultipartForm(config.MultipartMaxMemory)
			check := r.FormValue(config.CSRFInputName)
			if check != token {
				log.Error("CSRF mismatch! %s <> %s", check, token)
				templates.MakeErrorPage(
					"CSRF Error",
					"An error occurred while processing your request. Please go back and try again.",
					http.StatusForbidden,
				)(w, r.WithContext(ctx))
				return
			}
		}

		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

// MakeCSRFCookie gets or creates the CSRF cookie and returns its value.
func MakeCSRFCookie(r *http.Request, w http.ResponseWriter) string {
	// Has a token already?
	cookie, err := r.Cookie(config.CSRFCookieName)
	if err == nil {
		// log.Debug("MakeCSRFCookie: user has token %s", cookie.Value)
		return cookie.Value
	}

	// Generate a new CSRF token.
	token := uuid.New().String()
	cookie = &http.Cookie{
		Name:     config.CSRFCookieName,
		Value:    token,
		HttpOnly: true,
		Path:     "/",
	}
	// log.Debug("MakeCSRFCookie: giving cookie value %s to user", token)
	http.SetCookie(w, cookie)

	return token
}
