package admin

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Admin transparency page that lists the scopes and permissions an admin account has for all to see.
//
// This serves the routes `/admin/transparency` (index) and `/admin/transparency/{username}`
func Transparency() http.HandlerFunc {
	tmpl := templates.Must("admin/transparency.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Looking at a specific admin user, or index page (all admin users)?
		var (
			specificUser *models.User
			adminUsers   []*models.User
		)

		if username := r.PathValue("username"); username != "" {
			// Get this user.
			user, err := models.FindUsername(username)
			if err != nil {
				templates.NotFoundPage(w, r)
				return
			}

			// Only for admin user accounts.
			if !user.IsAdmin {
				templates.NotFoundPage(w, r)
				return
			}

			specificUser = user
		}

		// If not a specific user, load all of them for the index page.
		if specificUser == nil {
			if users, err := models.ListAdminUsers(); err != nil {
				session.FlashError(w, r, "Error listing admin users: %s", err)
			} else {
				adminUsers = users
			}
		}

		// Template variables.
		var vars = map[string]interface{}{
			"User":        specificUser,
			"AdminUsers":  adminUsers,
			"AdminScopes": config.ListAdminScopes(),
		}

		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
