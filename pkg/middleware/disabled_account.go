package middleware

import (
	"net/http"
	"strings"

	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/templates"
)

var tmplDisabledAccount = templates.Must("errors/disabled_account.html")

// Whitelist of paths to allow disabled accounts to access.
var disabledAccountPathWhitelist = []string{
	"/account/delete",
	"/account/reactivate",
}

// DisabledAccount check that limits a logged-in user's options to either reactivate their account,
// delete it, or log back out.
func DisabledAccount(currentUser *models.User, w http.ResponseWriter, r *http.Request) bool {
	// Is their account disabled?
	if currentUser.Status == models.UserStatusDisabled {
		// Whitelisted paths?
		for _, path := range disabledAccountPathWhitelist {
			if strings.HasPrefix(r.URL.Path, path) {
				return false
			}
		}

		// Show the disabled account page to all other requests.
		if err := tmplDisabledAccount.Execute(w, r, nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return true
	}

	return false
}
