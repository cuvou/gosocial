package settings

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/spam"
	"github.com/cuvou/gosocial/pkg/templates"
)

// StatusMessage settings page (/settings/status).
func StatusMessage() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get CurrentUser: %s", err)
			templates.Redirect(w, r.URL.Path)
			return
		}

		var (
			status     = r.PostFormValue("headline")
			expires, _ = strconv.Atoi(r.PostFormValue("expires"))
			intent     = r.PostFormValue("intent")
			nextURL    = "/me"
		)

		// Deleting their status?
		if intent == "delete" {
			status = ""
			expires = 0
		}

		// Validation.
		if len(status) > 60 {
			session.FlashError(w, r, "Your status message is too long (max 60 characters)")
			templates.Redirect(w, nextURL)
			return
		}
		if err := spam.DetectSpamLinks(status); err != nil {
			session.FlashError(w, r, err.Error())
			templates.Redirect(w, nextURL)
			return
		}

		if expires > 24*30 {
			expires = 24 * 30
		}

		// Store the expiration in terms of Unix time.
		var expireAt int64
		if expires > 0 {
			expireAt = time.Now().Unix() + int64(expires*3600)
		}

		currentUser.SetProfileField("headline", status)
		currentUser.SetProfileField("headline_expires", fmt.Sprintf("%d", expireAt))

		session.Flash(w, r, "Your status message has been updated!")
		templates.Redirect(w, nextURL)
	})
}
