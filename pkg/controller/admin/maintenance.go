package admin

import (
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Maintenance controller (/admin/maintenance)
func Maintenance() http.HandlerFunc {
	tmpl := templates.Must("admin/maintenance.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Query parameters.
		var (
			form   = r.FormValue("form") // Which form is submitted? maintenance vs. emergency, etc.
			intent = r.FormValue("intent")
		)

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get your current user: %s", err)
		}

		_ = currentUser

		// POST event handlers.
		if r.Method == http.MethodPost {

			// Which form was submitted?
			switch form {
			case "maintenance":
				// Maintenance Mode settings.
				var (
					headline         = r.PostFormValue("headline")
					message          = r.PostFormValue("message")
					onAllPages       = r.PostFormValue("all_pages") == "true"
					pauseSignup      = r.PostFormValue("signup") == "true"
					pauseLogin       = r.PostFormValue("login") == "true"
					pauseChat        = r.PostFormValue("chat") == "true"
					pauseInteraction = r.PostFormValue("interaction") == "true"
				)

				switch intent {
				case "everything", "nothing":
					pauseSignup = intent == "everything"
					pauseLogin = pauseSignup
					pauseChat = pauseSignup
					pauseInteraction = pauseSignup
					onAllPages = false
					intent = "save"
					fallthrough
				case "save":
					// Update and save the site settings.
					config.Current.Maintenance.Headline = headline
					config.Current.Maintenance.Message = message
					config.Current.Maintenance.MessageOnAllPages = onAllPages
					config.Current.Maintenance.PauseSignup = pauseSignup
					config.Current.Maintenance.PauseLogin = pauseLogin
					config.Current.Maintenance.PauseChat = pauseChat
					config.Current.Maintenance.PauseInteraction = pauseInteraction
					if err := config.WriteSettings(); err != nil {
						session.FlashError(w, r, "Couldn't write settings.json: %s", err)
					} else {
						session.Flash(w, r, "Maintenance settings updated!")
					}
				default:
					session.FlashError(w, r, "Unsupported intent: %s", intent)
				}
			case "emergency":
				// Emergency Kill Switch settings
				var (
					headline         = r.PostFormValue("headline")
					message          = r.PostFormValue("message")
					enabled          = r.PostFormValue("enabled") == "true"
					activated        = r.PostFormValue("activated") == "true"
					daysMissing, err = strconv.Atoi(r.PostFormValue("days_missing"))
				)

				if err != nil {
					session.FlashError(w, r, "Didn't save the Days Missing value: %s", err)
				}

				// Disabling it will also deactivate it.
				if !enabled {
					activated = false
				}

				config.Current.EmergencyKillSwitch.Enabled = enabled
				config.Current.EmergencyKillSwitch.Activated = activated
				config.Current.EmergencyKillSwitch.Headline = headline
				config.Current.EmergencyKillSwitch.Message = message
				config.Current.EmergencyKillSwitch.DaysMissingTTL = daysMissing

				if err := config.WriteSettings(); err != nil {
					session.FlashError(w, r, "Couldn't write settings.json: %s", err)
				} else {
					session.Flash(w, r, "Emergency Kill Switch settings updated!")
				}
			}

			templates.Redirect(w, r.URL.Path)
			return
		}

		// Get the owner's user for emergency kill switch UI.
		var owner *models.User
		if config.Current.EmergencyKillSwitch.OwnerUserID > 0 {
			if user, err := models.GetUser(config.Current.EmergencyKillSwitch.OwnerUserID); err == nil {
				owner = user
			}
		}

		var vars = map[string]interface{}{
			"Intent":     intent,
			"Maint":      config.Current.Maintenance,
			"KillSwitch": config.Current.EmergencyKillSwitch,
			"SiteOwner":  owner,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
