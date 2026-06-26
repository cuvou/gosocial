package comment

import (
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Subscription endpoint - to opt in or out of comment thread subscriptions.
func Subscription() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Query params.
		var (
			tableName = r.FormValue("table_name")
			tableID   uint64
			subscribe = r.FormValue("subscribe") == "true"
			fromURL   = r.FormValue("next") // what page to send back to
		)

		// Parse the table ID param.
		if idStr := r.FormValue("table_id"); idStr == "" {
			session.FlashError(w, r, "Comment table ID required.")
			templates.Redirect(w, "/")
			return
		} else {
			// Integer IDs in all other cases.
			if idInt, err := strconv.ParseUint(idStr, 10, 64); err != nil {
				session.FlashError(w, r, "Comment table ID invalid.")
				templates.Redirect(w, "/")
				return
			} else {
				tableID = idInt
			}
		}

		// Redirect URL must be relative.
		if !strings.HasPrefix(fromURL, "/") {
			// Maybe it's URL encoded?
			fromURL, _ = url.QueryUnescape(fromURL)
			if !strings.HasPrefix(fromURL, "/") {
				fromURL = "/"
			}
		}

		// Validate everything else.
		if _, ok := models.SubscribableTables[tableName]; !ok {
			session.FlashError(w, r, "You can not comment on that.")
			templates.Redirect(w, "/")
			return
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Language to use in the flash messages.
		var kind = "comments"

		// Get their subscription.
		sub, err := models.GetSubscription(currentUser, tableName, tableID)
		if err != nil {
			// If they want to subscribe, insert their row.
			if subscribe {
				if _, err := models.SubscribeTo(currentUser, tableName, tableID); err != nil {
					session.FlashError(w, r, "Couldn't create subscription: %s", err)
				} else {
					session.Flash(w, r, "You will now be notified about %s on this page.", kind)
				}
			} else {
				// An explicit subscribe=false, may be a preemptive opt-out as in
				// friend new photo notifications.
				if _, err := models.UnsubscribeTo(currentUser, tableName, tableID); err != nil {
					session.FlashError(w, r, "Couldn't create subscription: %s", err)
				} else {
					session.Flash(w, r, "You will no longer be notified about %s on this page.", kind)
				}
			}
		} else {
			// Toggle it.
			sub.Subscribed = subscribe
			if err := sub.Save(); err != nil {
				session.FlashError(w, r, "Couldn't save your subscription settings: %s", err)
			} else {
				if subscribe {
					session.Flash(w, r, "You will now be notified about %s on this page.", kind)
				} else {
					session.Flash(w, r, "You will no longer be notified about new %s on this page.", kind)
				}
			}
		}

		templates.Redirect(w, fromURL)
	})
}
