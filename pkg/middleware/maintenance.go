package middleware

import (
	"net/http"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/templates"
)

var tmplMaint = templates.Must("errors/maintenance.html")

// Whitelist of routes that Maintenance Mode permits while 'all interaction' is otherwise blocked.
var maintModeLockdownWhiteListedRoutes = []string{
	"/account/deactivate",
	"/account/reactivate",
	"/account/delete",
}

// MaintenanceMode check at the middleware level, e.g. to block
// LoginRequired and CertificationRequired if site-wide interaction
// is currently on hold. Returns true if handled.
func MaintenanceMode(currentUser *models.User, w http.ResponseWriter, r *http.Request) bool {
	// Is the site under a Maintenance Mode restriction?
	if (config.Current.Maintenance.PauseInteraction || config.Current.EmergencyKillSwitch.Activated) && !currentUser.IsAdmin {

		// Is it not a whitelisted route?
		for _, route := range maintModeLockdownWhiteListedRoutes {
			if strings.HasPrefix(r.URL.Path, route) {
				return false
			}
		}

		// Get the site owner in case this is an Emergency Kill Switch page.
		var owner *models.User
		if config.Current.EmergencyKillSwitch.OwnerUserID > 0 {
			if user, err := models.GetUser(config.Current.EmergencyKillSwitch.OwnerUserID); err == nil {
				owner = user
			}
		}

		// Show the maintenance gate page.
		var vars = map[string]interface{}{
			"Reason":    "interaction",
			"SiteOwner": owner,
		}
		if err := tmplMaint.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return true
	}

	return false
}

// SignupMaintenance may handle maintenance mode requests for signup gating.
func SignupMaintenance(w http.ResponseWriter, r *http.Request) bool {
	if config.Current.Maintenance.PauseSignup || config.Current.EmergencyKillSwitch.Activated {
		var vars = map[string]interface{}{
			"Reason": "signup",
		}
		if err := tmplMaint.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return true
	}
	return false
}

// LoginMaintenance may handle maintenance mode requests for login gating.
func LoginMaintenance(currentUser *models.User, w http.ResponseWriter, r *http.Request) bool {
	if config.Current.Maintenance.PauseLogin && !currentUser.IsAdmin {
		var vars = map[string]interface{}{
			"Reason": "login",
		}
		if err := tmplMaint.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return true
	}
	return false
}

// ChatMaintenance may handle maintenance mode requests for chat room gating.
func ChatMaintenance(currentUser *models.User, w http.ResponseWriter, r *http.Request) bool {
	if (config.Current.Maintenance.PauseChat || config.Current.EmergencyKillSwitch.Activated) && !currentUser.IsAdmin {
		var vars = map[string]interface{}{
			"Reason": "chat",
		}
		if err := tmplMaint.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return true
	}
	return false
}
