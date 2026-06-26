package settings

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Notification settings (/settings/notifications).
func Notifications() http.HandlerFunc {
	tmpl := templates.Must("settings/notifications.html")
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

			intent := r.PostFormValue("intent")
			switch intent {
			case "notifications":
				// Store their notification opt-outs.
				for _, key := range config.NotificationOptOutFields {
					var value = r.PostFormValue(key)

					// Boolean flip for DB storage:
					// - Pre-existing users before these options are added have no pref stored in the DB
					// - The default pref is opt-IN (receive all notifications)
					// - The checkboxes on front-end are on by default, uncheck them to opt-out, checkbox value="true"
					// - So when they post as "true" (default), we keep the notifications sending
					// - If they uncheck the box, no value is sent and that's an opt-out.
					switch value {
					case "":
						value = "true" // opt-out, store opt-out=true in the DB
					case "true":
						value = "false" // the box remained checked, they don't opt-out, store opt-out=false in the DB
					}

					// Save it. TODO: fires off inserts/updates for each one,
					// probably not performant to do.
					user.SetProfileField(key, value)
				}
				session.Flash(w, r, "Notification preferences updated!")

				// Save the user for new fields to be committed to DB.
				if err := user.Save(); err != nil {
					session.FlashError(w, r, "Failed to save user to database: %s", err)
				}

				// Are they unsubscribing from all threads?
				if r.PostFormValue("unsubscribe_all_threads") == "true" {
					if err := models.UnsubscribeAllThreads(user); err != nil {
						session.FlashError(w, r, "Couldn't unsubscribe from threads: %s", err)
					} else {
						session.Flash(w, r, "Unsubscribed from all comment threads!")
					}
				}
			case "push_notifications":
				// Store their notification opt-outs.
				for _, key := range config.PushNotificationOptOutFields {
					var value = r.PostFormValue(key)

					if value == "" {
						value = "true" // opt-out, store opt-out=true in the DB
					} else if value == "true" {
						value = "false" // the box remained checked, they don't opt-out, store opt-out=false in the DB
					}

					// Save it.
					user.SetProfileField(key, value)
				}
				session.Flash(w, r, "Notification preferences updated!")

				// Save the user for new fields to be committed to DB.
				if err := user.Save(); err != nil {
					session.FlashError(w, r, "Failed to save user to database: %s", err)
				}
			default:
				session.FlashError(w, r, "Unknown intent: %s", intent)
			}

			templates.Redirect(w, r.URL.Path)
			return
		}

		vars := map[string]interface{}{
			// Count of subscribed comment threads.
			"SubscriptionCount": models.CountSubscriptions(user),

			// Count of push notification subscriptions.
			"PushNotificationsCount": models.CountPushNotificationSubscriptions(user),
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
