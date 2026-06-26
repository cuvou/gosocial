package forum

import (
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Subscribe to a forum, adding it to your bookmark list.
func Subscribe() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the path parameters
		var (
			forumID, _ = strconv.ParseUint(r.FormValue("id"), 10, 64)
			fragment   = r.FormValue("fragment")
			forum      *models.Forum
			intent     = r.FormValue("intent")
		)

		// Look up the forum by its ID or fragment.
		if forumID > 0 {
			if found, err := models.GetForum(forumID); err != nil {
				templates.NotFoundPage(w, r)
				return
			} else {
				forum = found
			}
		} else if found, err := models.ForumByFragment(fragment); err != nil {
			templates.NotFoundPage(w, r)
			return
		} else {
			forum = found
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Is the forum about a Place?
		var (
			title    = forum.Title
			listName = "forum list"
			nextURL  = "/f/" + forum.Fragment
		)

		switch intent {
		case "follow":
			// Is it a private forum?
			if forum.Private && !currentUser.IsAdmin {
				templates.NotFoundPage(w, r)
				return
			}

			_, err := models.CreateForumMembership(currentUser.ID, forum.ID)
			if err != nil {
				session.FlashError(w, r, "Couldn't follow this forum: %s", err)
			} else {
				session.Flash(w, r, "You have added %s to your %s.", title, listName)
			}
		case "unfollow":
			fm, err := models.GetForumMembership(currentUser.ID, forum.ID)
			if err == nil {
				// Were we a moderator previously? If so, revoke the notification about it.
				if fm.IsModerator {
					if err := models.RemoveSpecificNotification(currentUser.ID, models.NotificationForumModerator, "forums", forum.ID); err != nil {
						log.Error("User unsubscribed from forum and couldn't remove their moderator notification: %s", err)
					}
				}

				err = fm.Delete()
				if err != nil {
					session.FlashError(w, r, "Couldn't delete your forum membership: %s", err)
				}
			}

			session.Flash(w, r, "You have removed %s from your %s.", title, listName)
		default:
			session.Flash(w, r, "Unknown intent.")
		}

		templates.Redirect(w, nextURL)
	})
}
