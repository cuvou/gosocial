package account

import (
	"net/http"
	"strings"

	"github.com/cuvou/gosocial/pkg/chat"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Deactivate account page (self service).
func Deactivate() http.HandlerFunc {
	tmpl := templates.Must("account/deactivate.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get your current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Confirm deletion.
		if r.Method == http.MethodPost {
			var password = strings.TrimSpace(r.PostFormValue("password"))
			if err := currentUser.CheckPassword(password); err != nil {
				session.FlashError(w, r, "You must enter your correct account password to delete your account.")
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Deactivate their account!
			currentUser.Status = models.UserStatusDisabled
			if err := currentUser.Save(); err != nil {
				session.FlashError(w, r, "Error while deactivating your account: %s", err)
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Sign them out.
			session.LogoutUser(w, r)
			session.Flash(w, r, "Your account has been deactivated and you are now logged out. If you wish to re-activate your account, sign in again with your username and password.")
			templates.Redirect(w, "/")

			// Maybe kick them from chat if this deletion makes them into a Shy Account.
			if _, err := chat.MaybeDisconnectUser(currentUser); err != nil {
				log.Error("chat.MaybeDisconnectUser(%s#%d): %s", currentUser.Username, currentUser.ID, err)
			}

			// Log the change.
			models.LogEvent(currentUser, nil, models.ChangeLogLifecycle, "users", currentUser.ID, "Deactivated their account.")
			return
		}

		var vars = map[string]interface{}{}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

// Reactivate account page
func Reactivate() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get your current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		if currentUser.Status != models.UserStatusDisabled {
			session.FlashError(w, r, "Your account was not disabled in the first place!")
			templates.Redirect(w, "/")
			return
		}

		// Reactivate them.
		currentUser.Status = models.UserStatusActive
		if err := currentUser.Save(); err != nil {
			session.FlashError(w, r, "Error while reactivating your account: %s", err)
			templates.Redirect(w, "/")
			return
		}

		session.Flash(w, r, "Welcome back! Your account has been reactivated.")
		templates.Redirect(w, "/")

		// Log the change.
		models.LogEvent(currentUser, nil, models.ChangeLogLifecycle, "users", currentUser.ID, "Reactivated their account.")
	})
}
