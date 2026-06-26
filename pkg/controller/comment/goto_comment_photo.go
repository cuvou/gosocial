package comment

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// GoToCommentPhoto finds the correct link to view the thread that a CommentPhoto appears in.
//
// Note: the user can only deep link into their own comment photos.
//
// This is to support the "My Media" page.
func GoToCommentPhoto() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Query params.
		var (
			photoID uint64
		)

		// Parse the ID param.
		if idStr := r.FormValue("id"); idStr == "" {
			session.FlashError(w, r, "ID required.")
			templates.Redirect(w, "/")
			return
		} else {
			if idInt, err := strconv.Atoi(idStr); err != nil {
				session.FlashError(w, r, "ID invalid.")
				templates.Redirect(w, "/")
				return
			} else {
				photoID = uint64(idInt)
			}
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Locate this comment photo.
		comment, err := models.GetCommentPhoto(photoID)
		if err != nil {
			session.FlashError(w, r, "Couldn't find that comment photo: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Validate ownership of the comment photo so users don't enumerate forum comments.
		if !currentUser.IsAdmin && currentUser.ID != comment.UserID {
			session.FlashError(w, r, "That comment photo does not belong to you.")
			templates.Redirect(w, "/")
			return
		}

		// Redirect the user to its comment.
		templates.Redirect(w, fmt.Sprintf("/go/comment?id=%d", comment.CommentID))
	})
}
