package inbox

import (
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Delete a new chat coming from a user's profile page.
func Delete() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			session.FlashError(w, r, "Invalid method.")
			templates.Redirect(w, "/")
			return
		}

		// Parse parameters.
		var (
			id        uint64
			idStr     = r.FormValue("id")
			deleteAll = r.FormValue("intent") == "delete-thread"
			next      = r.FormValue("next")
		)

		if value, err := strconv.Atoi(idStr); err == nil {
			id = uint64(value)
		} else {
			session.FlashError(w, r, "Request error.")
			templates.Redirect(w, "/")
			return
		}

		// The redirect URL must be local.
		if len(next) == 0 || next[0] != '/' {
			next = "/"
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Error getting the current user: %s", err)
			templates.Redirect(w, next)
			return
		}

		// Lookup the message.
		message, err := models.GetMessage(id)
		if err != nil {
			session.FlashError(w, r, err.Error())
			templates.Redirect(w, next)
		}

		// We should be a party on this message.
		if deleteAll {
			if message.SourceUserID != currentUser.ID &&
				message.TargetUserID != currentUser.ID {
				session.FlashError(w, r, "That is not your conversation thread.")
				templates.Redirect(w, next)
				return
			}
		} else if message.SourceUserID != currentUser.ID {
			session.FlashError(w, r, "You did not create that message so you can't delete it.")
			templates.Redirect(w, next)
			return
		}

		// Delete whole thread?
		if deleteAll {
			if err := models.DeleteMessageThread(message); err != nil {
				session.FlashError(w, r, "Error removing thread: %s", err)
			} else {
				session.Flash(w, r, "Message thread has been removed.")
			}
			templates.Redirect(w, next)
			return
		}

		// Do the needful.
		if err := message.Delete(); err != nil {
			session.FlashError(w, r, "Error deleting the message: %s", err)
		} else {
			session.Flash(w, r, "Message deleted!")
		}

		templates.Redirect(w, next)
	})
}
