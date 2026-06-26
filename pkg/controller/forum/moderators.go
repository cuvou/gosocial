package forum

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// ManageModerators controller (/forum/admin/moderators) to appoint moderators to your (user) forum.
func ManageModerators() http.HandlerFunc {
	// Reuse the upload page but with an EditPhoto variable.
	tmpl := templates.Must("forum/moderators.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			intent   = r.FormValue("intent")
			stringID = r.FormValue("forum_id")
		)

		// Parse forum_id query parameter.
		var forumID uint64
		if stringID != "" {
			if i, err := strconv.Atoi(stringID); err == nil {
				forumID = uint64(i)
			} else {
				session.FlashError(w, r, "Edit parameter: forum_id was not an integer")
				templates.Redirect(w, "/forum/admin")
				return
			}
		}

		// Redirect URLs
		var (
			next         = fmt.Sprintf("%s?forum_id=%d", r.URL.Path, forumID)
			nextFinished = fmt.Sprintf("/forum/admin/edit?id=%d", forumID)
		)

		// Load the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: could not get currentUser.")
			templates.Redirect(w, "/")
			return
		}

		// Are we adding/removing a user as moderator?
		var (
			username = r.FormValue("to")
			user     *models.User
		)
		if username != "" {
			if found, err := models.FindUsername(username); err != nil {
				templates.NotFoundPage(w, r)
				return
			} else {
				user = found
			}
		}

		// Look up the forum by its fragment.
		forum, err := models.GetForum(forumID)
		if err != nil {
			templates.NotFoundPage(w, r)
			return
		}

		// User must be the owner of this forum, or a privileged admin.
		if !forum.CanEdit(currentUser) {
			templates.ForbiddenPage(w, r)
			return
		}

		// The forum owner can not add themself.
		if user != nil && forum.OwnerID == user.ID {
			session.FlashError(w, r, "You can not add the forum owner to its moderators list.")
			templates.Redirect(w, next)
			return
		}

		// POSTing?
		if r.Method == http.MethodPost {
			switch intent {
			case "submit":
				// Confirmed adding a moderator.
				if _, err := forum.AddModerator(user); err != nil {
					session.FlashError(w, r, "Error adding the moderator: %s", err)
					templates.Redirect(w, next)
					return
				}

				// Create a notification for this.
				notif := &models.Notification{
					UserID:    user.ID,
					AboutUser: *currentUser,
					Type:      models.NotificationForumModerator,
					TableName: "forums",
					TableID:   forum.ID,
					Link:      fmt.Sprintf("/f/%s", forum.Fragment),
				}
				if err := models.CreateNotification(notif); err != nil {
					log.Error("Couldn't create PrivatePhoto notification: %s", err)
				}

				session.Flash(w, r, "%s has been added to the moderators list!", user.Username)
				templates.Redirect(w, nextFinished)
				return
			case "confirm-remove":
				// Confirm removing a moderator.
				if _, err := forum.RemoveModerator(user); err != nil {
					session.FlashError(w, r, "Error removing the moderator: %s", err)
					templates.Redirect(w, next)
					return
				}

				// Revoke any past notifications they had about being added as moderator.
				if err := models.RemoveSpecificNotification(user.ID, models.NotificationForumModerator, "forums", forum.ID); err != nil {
					log.Error("Couldn't revoke the forum moderator notification: %s", err)
				}

				session.Flash(w, r, "%s has been removed from the moderators list.", user.Username)
				templates.Redirect(w, nextFinished)
				return
			}
		}

		var vars = map[string]interface{}{
			"Forum":      forum,
			"User":       user,
			"IsRemoving": intent == "remove",
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

// ModerateThread endpoint - perform a mod action like pinning or locking a thread.
func ModerateThread() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Query params.
		var (
			threadID, err = strconv.Atoi(r.PathValue("id"))
			intent        = r.PostFormValue("intent")
			nextURL       = fmt.Sprintf("/forum/thread/%d", threadID)
		)

		if err != nil {
			session.FlashError(w, r, "Invalid thread ID.")
			templates.Redirect(w, nextURL)
			return
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Get this thread.
		thread, err := models.GetThread(uint64(threadID))
		if err != nil {
			templates.NotFoundPage(w, r)
			return
		}

		// Get its forum.
		forum, err := models.GetForum(thread.ForumID)
		if err != nil {
			templates.NotFoundPage(w, r)
			return
		}

		// User must at least be able to moderate.
		if !forum.CanBeModeratedBy(currentUser) {
			templates.ForbiddenPage(w, r)
			return
		}

		// Does the user have Ownership level access (including privileged admins)
		var isOwner = forum.OwnerID == currentUser.ID || currentUser.HasAdminScope(config.ScopeForumAdmin)

		/****
		 * Moderator level permissions.
		 ***/
		switch intent {
		case "lock":
			thread.NoReply = true
			session.Flash(w, r, "This thread has been locked and will not be accepting any new replies.")
		case "unlock":
			thread.NoReply = false
			session.Flash(w, r, "This thread has been unlocked and can accept new replies again.")
		default:
			if !isOwner {
				// End of the road.
				templates.ForbiddenPage(w, r)
				return
			}
		}

		/****
		 * Owner + Admin level permissions.
		 ***/
		switch intent {
		case "pin":
			thread.Pinned = true
			session.Flash(w, r, "This thread is now pinned to the top of the forum.")
		case "unpin":
			thread.Pinned = false
			session.Flash(w, r, "This thread will no longer be pinned to the top of the forum.")
		case "explicit":
			thread.Explicit = true
			session.Flash(w, r, "This thread has been marked as Explicit.")
		case "unexplicit":
			thread.Explicit = false
			session.Flash(w, r, "The Explicit tag has been removed from this thread.")
		case "lock", "unlock":
			// Handled above (moderator level permission).
		default:
			session.FlashError(w, r, "Unknown moderation action: %s", intent)
		}

		// Save changes to the thread, without pinging its UpdatedAt.
		if err := thread.SaveModeration(); err != nil {
			session.FlashError(w, r, "Error saving thread: %s", err)
		}

		templates.Redirect(w, nextURL)
	})
}
