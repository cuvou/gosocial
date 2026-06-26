package htmx

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/middleware"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/utility"
	"github.com/google/uuid"
)

// CommentThread for common frontend shared between photos, blogs, videos, etc.
func CommentThread() http.HandlerFunc {
	tmpl := templates.MustLoadCustom("partials/htmx/comments.html")

	var sortWhitelist = []string{
		"created_at asc",
		"created_at desc",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// HTMX widget parameters.
		var (
			// commentCount
			tableName     = r.FormValue("table_name")
			tableIntID, _ = strconv.Atoi(r.FormValue("table_id"))
			tableID       = uint64(tableIntID)
			sort          = utility.StringIn(r.FormValue("sort"), sortWhitelist, sortWhitelist[0])
			nextURL       = r.FormValue("next")
			message       = r.FormValue("message") // drafted message, for HTMX re-render on sort
			uid           = uuid.New().String()

			// No new comments accepted.
			disabled = r.FormValue("disabled") == "true"

			// If the frontend passes an admin comment warning
			// (e.g. 'this content is private and you would not have been allowed to see it normally')
			adminWarning = r.FormValue("admin_warning") == "true"

			// The owner of the thing being commented on.
			threadOwner *models.User
		)

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			w.Write([]byte("You must be logged in to view this page."))
			return
		}

		// Is the site under a Maintenance Mode restriction?
		if middleware.MaintenanceMode(currentUser, w, r) {
			return
		}

		// Find out who the owner of this comment thread is.
		var ownerUserID uint64
		switch tableName {
		case "photos":
			if p, err := models.GetPhoto(tableID); err != nil {
				w.Write([]byte("Photo Not Found"))
				return
			} else {
				ownerUserID = p.UserID
			}
		default:
			w.Write([]byte("Unsupported table name for comments: " + tableName))
			return
		}

		// Look up the comment thread owner.
		if ownerUserID == 0 {
			w.Write([]byte("Comment thread owner is unknown."))
			return
		} else {
			if u, err := models.GetUser(ownerUserID); err != nil {
				w.Write([]byte("Couldn't find the owner for this comment thread."))
				return
			} else {
				threadOwner = u
			}
		}

		// Get the comments.
		comments, err := models.ListComments(currentUser, tableName, tableID, sort)
		if err != nil {
			fmt.Fprintf(w, "Error listing comments: %s", err)
		}

		// Populate the user relationships in these comments.
		models.SetUserRelationshipsInComments(currentUser, comments)

		// Map likes.
		var commentIDs = []uint64{}
		for _, c := range comments {
			commentIDs = append(commentIDs, c.ID)
		}
		likeMap := models.MapLikes(currentUser, "comments", commentIDs)

		// User notification subscription settings.
		var (
			isOwnedThread   = threadOwner.ID == currentUser.ID
			canSubscribe    = threadOwner.ID != currentUser.ID
			_, isSubscribed = models.IsSubscribed(currentUser, tableName, tableID)
		)

		var (
			buf  = bytes.NewBuffer([]byte{})
			vars = map[string]interface{}{
				"IsOwnedThread":    isOwnedThread,
				"TableName":        tableName,
				"TableID":          tableID,
				"CommentsDisabled": disabled,
				"Comments":         comments,
				"LikeMap":          likeMap,
				"CanSubscribe":     canSubscribe,
				"IsSubscribed":     isSubscribed,
				"Message":          message,
				"Sort":             sort,
				"NextURL":          nextURL,

				"AdminCommentWarning": adminWarning,

				// A unique ID for the HTML component, to allow self-reloads with the Sort field.
				"UniqueID": uid,
			}
		)
		if err := tmpl.Execute(buf, r, vars); err != nil {
			fmt.Fprintf(w, "[template error: %s]", err)
			return
		}

		w.Write(buf.Bytes())
	})
}
