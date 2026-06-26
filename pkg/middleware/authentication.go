package middleware

import (
	"context"
	"net/http"
	"net/url"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

/*
RunLoginTasks is called by the LoginRequired/CertRequired middleware to run
tasks periodically that are triggered by a user's login.

This is NOT run when a user is being impersonated.

Periodically:

- Ping their LastLoginAt time (with a cooldown)
- Update their location if using GeoIP for location source.

Always:

- Log their IP addresses on all visits.
*/
func RunLoginTasks(r *http.Request, user *models.User) error {
	var (
		// With a cooldown period, check if this is a new 'login' to run periodic tasks.
		isNewLogin = time.Since(user.LastLoginAt) > config.LastLoginAtCooldown
	)

	// Once per 'login'
	if isNewLogin {

		// Ping our 'Last Logged In' date.
		if err := user.PingLastLoginAt(); err != nil {
			log.Error("LoginRequired: couldn't refresh LastLoginAt for user %s: %s", user.Username, err)
		}

		// If our location is set by GeoIP, refresh it now.
		if _, err := models.RefreshGeoIP(user.ID, r); err != nil {
			log.Error("LoginRequired: RefreshGeoIP(%d): %s", user.ID, err)
		}

		// Check if the user is eligible to be shown the Security Checkup (they have not
		// set up 2FA yet). On the next AgeGate middleware page (e.g. login required pages
		// outside of a whitelist of routes), the Security Checkup will be shown.
		if user.SecurityCheckupEligible(false) {
			// Set the flag for the 'hard' interstitial. On the next LoginRequired screen, they are
			// forced to interact with the page (they can 'Remind me later (30 days)')
			user.SetProfileField("security_checkup_eligible", "true")
		}
	}

	// Log the last visit of their current IP address, always.
	// Increment the visit count from this address if LastLoginAt was pinged.
	if err := models.PingIPAddress(r, user, isNewLogin); err != nil {
		log.Error("LoginRequired: couldn't ping user %s IP address: %s", user.Username, err)
	}

	return nil
}

// LoginRequired middleware.
func LoginRequired(handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// User must be logged in.
		user, err := session.CurrentUser(r)
		if err != nil {
			log.Error("LoginRequired: %s", err)
			session.FlashError(w, r, "You must be signed in to view this page.")
			templates.Redirect(w, "/login?next="+url.QueryEscape(r.URL.String()))
			return
		}

		// Are they banned?
		if user.Status == models.UserStatusBanned {
			session.LogoutUser(w, r)
			session.FlashError(w, r, "Your account has been banned and you are now logged out.")
			templates.Redirect(w, "/")
			return
		}

		// Is their account disabled? Whitelist only the endpoints to reactivate or delete.
		if DisabledAccount(user, w, r) {
			return
		}

		// Is the site under a Maintenance Mode restriction?
		if MaintenanceMode(user, w, r) {
			return
		}

		// Run the user's periodic login tasks, but not if impersonated.
		if !session.Impersonated(r) {
			if err := RunLoginTasks(r, user); err != nil {
				log.Error("LoginRequired: couldn't RunLoginTasks: %s", err)
			}
		}

		// Ask the user for their birthdate?
		if AgeGate(user, w, r) {
			return
		}

		// Stick the CurrentUser in the request context so future calls to session.CurrentUser can read it.
		ctx := context.WithValue(r.Context(), session.CurrentUserKey, user)
		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}

// AdminRequired middleware.
func AdminRequired(scope string, handler http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// User must be logged in.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			log.Error("AdminRequired: %s", err)
			session.FlashError(w, r, "You must be signed in to view this page.")
			templates.Redirect(w, "/login?next="+url.QueryEscape(r.URL.String()))
			return
		}

		// Stick the CurrentUser in the request context so future calls to session.CurrentUser can read it.
		ctx := context.WithValue(r.Context(), session.CurrentUserKey, currentUser)

		// Admin required.
		if !currentUser.IsAdmin {
			errhandler := templates.MakeErrorPage("Admin Required", "You do not have permission for this page.", http.StatusForbidden)
			errhandler.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		// Ensure the admin scope.
		if scope != "" && !currentUser.HasAdminScope(scope) {
			errhandler := templates.MakeErrorPage(
				"Admin Scope Required",
				"Missing required admin scope: "+scope,
				http.StatusForbidden,
			)
			errhandler.ServeHTTP(w, r.WithContext(ctx))
			return
		}

		handler.ServeHTTP(w, r.WithContext(ctx))
	})
}
