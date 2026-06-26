package inbox

import (
	"fmt"
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/webpush"
)

// Compose a new chat coming from a user's profile page.
func Compose() http.HandlerFunc {
	tmpl := templates.Must("inbox/compose.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// To whom?
		username := r.FormValue("to")
		user, err := models.FindUsername(username)
		if err != nil {
			templates.NotFoundPage(w, r)
			return
		}

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: could not get currentUser.")
			templates.Redirect(w, "/")
			return
		}

		if currentUser.ID == user.ID {
			session.FlashError(w, r, "You cannot send a message to yourself.")
			templates.Redirect(w, "/messages")
			return
		}

		// Any blocking?
		if models.IsBlocking(currentUser.ID, user.ID) && !currentUser.IsAdmin {
			session.FlashError(w, r, "You are blocked from sending a message to this user.")
			templates.Redirect(w, "/messages")
			return
		}

		// POSTing?
		if r.Method == http.MethodPost {
			var (
				message = r.FormValue("message")
				from    = r.FormValue("from") // e.g. "inbox", default "profile", where to redirect to
			)
			if len(message) == 0 {
				session.FlashError(w, r, "A message is required.")
				templates.Redirect(w, r.URL.Path+"?to="+username)
				return
			}

			// Post it!
			m, err := models.SendMessage(currentUser.ID, user.ID, message)
			if err != nil {
				session.FlashError(w, r, "Failed to create the message in the database: %s", err)
				templates.Redirect(w, r.URL.Path+"?to="+username)
				return
			}

			// Send a push notification to the recipient.
			go func() {
				// Opted out of this one?
				if user.GetProfileField(config.PushNotificationOptOutMessage) == "true" {
					return
				}

				log.Info("Try and send Web Push notification about new Message to: %s", user.Username)
				webpush.SendNotification(user, webpush.Payload{
					Topic: "inbox",
					Title: "New Message!",
					Body:  fmt.Sprintf("%s has left you a message on %s.", currentUser.Username, config.Title),
				})
			}()

			session.Flash(w, r, "Your message has been delivered!")
			if from == "inbox" {
				templates.Redirect(w, fmt.Sprintf("/messages/read/%d", m.ID))
				return
			}
			templates.Redirect(w, "/messages")
			return
		}

		// If we already have a thread open with them, go to that instead of the stand-alone compose page.
		if msgID, ok := models.HasMessageThread(currentUser, user); ok {
			templates.Redirect(w, fmt.Sprintf("/messages/read/%d", msgID))
			return
		}

		// On GET request (come from a user profile page):
		// Do not allow a shy user to initiate DMs with a non-shy one.
		var (
			areFriends = models.AreFriends(currentUser.ID, user.ID)
		)

		// Does the recipient have a privacy control on their DMs?
		privacySetting := models.GetPrivacySetting(user.ID)
		switch privacySetting.FirstMessages {
		case "friends":
			if !areFriends && !currentUser.IsAdmin {
				session.FlashError(w, r, "This user only wants to receive new DMs from their friends.")
				templates.Redirect(w, "/u/"+user.Username)
				return
			}
		case "nobody":
			if !currentUser.IsAdmin {
				session.FlashError(w, r, "This user's DMs are closed and they do not want any new conversations.")
				templates.Redirect(w, "/u/"+user.Username)
				return
			}
		}

		var vars = map[string]interface{}{
			"User": user,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
