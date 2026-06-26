package index

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/models/demographic"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Demographics page (/insights) to show a peek at website demographics.
func Demographics() http.HandlerFunc {
	tmpl := templates.Must("demographics.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			refresh = r.FormValue("refresh") == "true"
		)

		// Are we refreshing? Check if an admin is logged in.
		if refresh {
			currentUser, err := session.CurrentUser(r)
			if err != nil {
				session.FlashError(w, r, "You must be logged in to do that!")
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Do the refresh?
			if currentUser.IsAdmin {
				_, err := demographic.Refresh()
				if err != nil {
					session.FlashError(w, r, "Refreshing the insights: %s", err)
				}
			}

			templates.Redirect(w, r.URL.Path)
			return
		}

		// Get website statistics to show on home page.
		demo, err := demographic.Get()
		if err != nil {
			session.FlashError(w, r, "Couldn't get website statistics: %s", err)
			templates.Redirect(w, "/")
			return
		}

		vars := map[string]interface{}{
			"Demographic": demo,
		}

		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
