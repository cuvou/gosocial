package account

import (
	"net/http"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Security Checkup page that prompts the user to configure Two Factor Auth once in a while.
func SecurityCheckup() http.HandlerFunc {
	tmpl := templates.Must("account/security_checkup.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			nextURL      = r.FormValue("next")
			isLoginEvent = r.FormValue("login") == "true"
		)

		if !strings.HasPrefix(nextURL, "/") {
			nextURL = "/me"
		}

		// Load the current user in case of updates.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get CurrentUser: %s", err)
			templates.Redirect(w, r.URL.Path)
			return
		}

		// 2FA settings.
		tf := models.Get2FA(currentUser.ID)

		// Number of recommended actions (only 2FA for now, and always Login Sessions).
		var recommendedActions = 1
		if !tf.Enabled {
			recommendedActions++
		}

		// Are we POSTing?
		if r.Method == http.MethodPost {

			// Don't ask them again for 30 days.
			ttl := time.Now().Add(time.Duration(config.SecurityCheckupCooldownDaysHard) * 24 * time.Hour).Format(time.RFC3339Nano)

			// If this was not a Login Event interstitial, clear the eligible flag and set a reminder.
			if !isLoginEvent {
				currentUser.SetProfileField("security_checkup_not_before", ttl)
				currentUser.DeleteProfileField("security_checkup_eligible")
			}

			templates.Redirect(w, nextURL)
			return
		}

		// If this was the soft interstitial (login event), ping the shorter cooldown timer.
		if isLoginEvent {
			ttl := time.Now().Add(time.Duration(config.SecurityCheckupCooldownDaysSoft) * 24 * time.Hour).Format(time.RFC3339Nano)
			currentUser.SetProfileField("security_checkup_not_before_soft", ttl)
		}

		vars := map[string]interface{}{
			"RecommendedActions":          recommendedActions,
			"TwoFactor":                   tf,
			"LoginSessionCount":           models.CountLoginSessions(currentUser),
			"NextURL":                     nextURL,
			"SecurityCheckupCooldownDays": config.SecurityCheckupCooldownDaysHard,
			"IsLoginEvent":                isLoginEvent,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
