package admin

import (
	"fmt"
	"net/http"
	"sort"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/webpush"
)

// BulkMessage to send a DM to many people at once.
func BulkMessage() http.HandlerFunc {
	tmpl := templates.Must("admin/bulk_message.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// To whom?
		var (
			usernames   = []string{}
			toList      = r.FormValue("to")
			targetUsers = []*models.User{}
		)
		if toList != "" {
			usernames = strings.Split(
				strings.ReplaceAll(r.FormValue("to"), " ", ""),
				",",
			)
		}
		if len(usernames) > 0 {
			users, err := models.MapUsersByUsername(usernames)
			if err != nil || len(users) != len(usernames) {
				session.FlashError(w, r, "Didn't find all those usernames (%d of %d found): %s", len(users), len(usernames), err)
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Sort them by username.
			sort.Strings(usernames)
			for _, username := range usernames {
				if user, ok := users[username]; ok {
					targetUsers = append(targetUsers, user)
				}
			}
		}

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: could not get currentUser.")
			templates.Redirect(w, "/")
			return
		}

		// POSTing?
		if r.Method == http.MethodPost {
			var (
				message = r.FormValue("message")
			)
			if len(message) == 0 {
				session.FlashError(w, r, "A message is required.")
				templates.Redirect(w, r.URL.Path+"?to="+r.FormValue("to"))
				return
			}

			// Send the messages! In a background thread in case there are a lot of them.
			go func() {
				for _, user := range targetUsers {
					if user.ID == currentUser.ID {
						// Skip messages to ourself.
						continue
					}

					_, err := models.SendMessage(currentUser.ID, user.ID, message)
					if err != nil {
						session.FlashError(w, r, "Failed to create the message in the database for %s: %s", user.Username, err)
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
				}
			}()

			session.Flash(w, r, "Your message has been delivered!")
			templates.Redirect(w, "/messages")
			return
		}

		var vars = map[string]interface{}{
			"Users": targetUsers,
			"To":    r.FormValue("to"),
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
