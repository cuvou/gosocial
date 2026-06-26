package account

import (
	"net/http"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/utility"
)

// WebsiteTheme is a dedicated page for setting website theme preferences.
func WebsiteTheme() http.HandlerFunc {
	tmpl := templates.Must("account/website_theme.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var next = r.FormValue("next")
		if !strings.HasPrefix(next, "/") {
			next = r.URL.Path
		}

		// Load the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get CurrentUser: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Are we POSTing?
		if r.Method == http.MethodPost {

			var (
				lightDark = r.PostFormValue("website-theme")
				hue       = r.PostFormValue("website-theme-hue")
			)

			// Constrain values.
			lightDark = utility.StringIn(lightDark, []string{"light", "dark", ""}, "")
			hue = utility.StringInOptGroup(hue, config.WebsiteThemeHueChoices, "")

			currentUser.SetProfileField("website-theme", lightDark)
			currentUser.SetProfileField("website-theme-hue", hue)

			templates.Redirect(w, next)
			return

		}

		vars := map[string]interface{}{
			"WebsiteThemeHueChoices": config.WebsiteThemeHueChoices,
			"NextURL":                next,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
