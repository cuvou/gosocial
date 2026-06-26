package account

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cuvou/gosocial/pkg/chat"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/models/deletion"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Delete account page (self service).
func Delete() http.HandlerFunc {
	tmpl := templates.Must("account/delete.html")
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

			// Delete their account!
			if err := deletion.DeleteUser(currentUser); err != nil {
				session.FlashError(w, r, "Error while deleting your account: %s", err)
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Did they leave feedback for us?
			if feedback := strings.TrimSpace(r.PostFormValue("feedback")); feedback != "" {
				fb := &models.Feedback{
					Intent:  "contact",
					Subject: "Deleted Account Feedback",
					Message: fmt.Sprintf(
						"The username **@%s** has deleted their account and left the following feedback message:\n\n%s",
						currentUser.Username,
						feedback,
					),
				}

				// Save the feedback.
				if err := models.CreateFeedback(fb); err != nil {
					log.Error("Couldn't save feedback from user deleting account: %s", err)
				}
			}

			// Sign them out.
			session.LogoutUser(w, r)
			session.Flash(w, r, "Your account has been deleted.")
			templates.Redirect(w, "/")

			// Kick them from the chat room if they are online.
			if _, err := chat.DisconnectUserNow(currentUser, "You have been signed out of chat because you had deleted your account."); err != nil {
				log.Error("chat.DisconnectUserNow(%s#%d): %s", currentUser.Username, currentUser.ID, err)
			}

			// Log the change.
			models.LogDeleted(nil, nil, "users", currentUser.ID, fmt.Sprintf("Username %s has deleted their account.", currentUser.Username), nil)
			return
		}

		var vars = map[string]interface{}{}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
