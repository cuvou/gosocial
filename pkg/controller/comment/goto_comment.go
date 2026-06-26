package comment

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// GoToComment finds the correct link to view a comment.
func GoToComment() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Query params.
		var (
			commentID uint64
		)

		// Parse the ID param.
		if idStr := r.FormValue("id"); idStr == "" {
			session.FlashError(w, r, "Comment ID required.")
			templates.Redirect(w, "/")
			return
		} else {
			if idInt, err := strconv.Atoi(idStr); err != nil {
				session.FlashError(w, r, "Comment ID invalid.")
				templates.Redirect(w, "/")
				return
			} else {
				commentID = uint64(idInt)
			}
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Locate this comment.
		comment, err := models.GetComment(commentID)
		if err != nil {
			session.FlashError(w, r, "Couldn't find that comment: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Where is this comment at?
		switch comment.TableName {
		case "threads":
			// Find what page it is on.
			page, err := models.FindPageByComment(currentUser, comment, config.PageSizeThreadList)
			if err != nil {
				session.FlashError(w, r, "Couldn't find that comment, but bringing you to the forum thread instead.")
				page = 1
			}

			templates.Redirect(w, fmt.Sprintf("/forum/thread/%d?page=%d#p%d", comment.TableID, page, comment.ID))
			return
		case "photos":
			// A photo comment thread, easy: only one page.
			templates.Redirect(w, fmt.Sprintf("/photo/view?id=%d#p%d", comment.TableID, comment.ID))
			return
		default:
			session.FlashError(w, r, "Unknown type of comment.")
		}

		templates.Redirect(w, "/")
	})
}
