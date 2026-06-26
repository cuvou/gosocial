package settings

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Website Preferences (/settings/prefs).
func Prefs() http.HandlerFunc {
	tmpl := templates.Must("settings/prefs.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Load the current user in case of updates.
		user, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get CurrentUser: %s", err)
			templates.Redirect(w, r.URL.Path)
			return
		}

		// Are we POSTing?
		if r.Method == http.MethodPost {
			var (
				explicit     = r.PostFormValue("explicit") == "true"
				blurExplicit = r.PostFormValue("blur_explicit")
				autoplayGif  = r.PostFormValue("autoplay_gif")
			)

			user.Explicit = explicit

			// Set profile field prefs.
			user.SetProfileField("blur_explicit", blurExplicit)
			if autoplayGif != "true" {
				autoplayGif = "false"
			}
			user.SetProfileField("autoplay_gif", autoplayGif)

			if err := user.Save(); err != nil {
				session.FlashError(w, r, "Failed to save user to database: %s", err)
			}

			session.Flash(w, r, "Website preferences updated!")
			templates.Redirect(w, r.URL.Path)
			return
		}

		vars := map[string]interface{}{}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
