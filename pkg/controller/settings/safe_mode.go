package settings

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// SafeMode toggle page (/settings/safe-mode).
func SafeMode() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			next, _ = url.QueryUnescape(r.FormValue("next"))
			ses     = session.Get(r)
			flash   = "Safe mode has been enabled. All pictures on the main website will be blurred until clicked."
		)

		log.Error("next URL: %s", next)

		if !strings.HasPrefix(next, "/") {
			next = "/"
		}

		ses.SafeMode = !ses.SafeMode
		ses.Save(w, r)

		if !ses.SafeMode {
			flash = "Safe mode has been turned off. Pictures on the site will again display as normal."
		}

		session.Flash(w, r, flash)
		templates.Redirect(w, next)
	})
}
