package comment

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	pphoto "github.com/cuvou/gosocial/pkg/photo"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// BatchEdit controller (/comment/batch-edit?id=N)
//
// This is called by the "My Media" page for the user to batch delete CommentPhotos
// that they had posted on the forums.
//
// It is a simplified version of the photo.BatchEdit controller, but with only delete
// capability and it operates on CommentPhotos instead.
func BatchEdit() http.HandlerFunc {
	tmpl := templates.Must("photo/batch_edit.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			// Form params
			intent      = r.FormValue("intent")
			photoIDs    []uint64
			redirectURI = "/photo/media?view=forum"
		)

		if intent != "delete" {
			session.FlashError(w, r, "Unsupported intent.")
			templates.Redirect(w, "/photo/media")
			return
		}

		// Collect the photo ID params.
		if value, ok := r.Form["id"]; ok {
			for _, idStr := range value {
				if photoID, err := strconv.Atoi(idStr); err == nil {
					photoIDs = append(photoIDs, uint64(photoID))
				} else {
					log.Error("parsing photo ID %s: %s", idStr, err)
				}
			}
		}

		// Validation.
		if len(photoIDs) == 0 || len(photoIDs) > 100 {
			session.FlashError(w, r, "Invalid number of photo IDs.")
			templates.Redirect(w, "/")
			return
		}

		// Find these photos by ID.
		photos, err := models.GetCommentPhotos(photoIDs)
		if err != nil {
			templates.NotFoundPage(w, r)
			return
		}

		// Load the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: couldn't get CurrentUser")
			templates.Redirect(w, "/")
			return
		}

		// Validate permission to edit all of these photos.
		var commentIDs []uint64
		for _, photo := range photos {
			if !photo.CanBeEditedBy(currentUser) {
				templates.ForbiddenPage(w, r)
				return
			}
			commentIDs = append(commentIDs, photo.CommentID)
		}

		// Map the parent comments that these photos belonged to.
		comments, err := models.GetComments(commentIDs)
		if err != nil {
			log.Error("Couldn't get comments for these photos: %s", err)
		}

		// If any of these comments are top-levels on forum threads, we don't want to
		// try and delete them (which will fail on Postgres), but to act like the comments
		// had text bodies (even if they didn't) and show the notice of deleted attachment.
		threadMap := models.MapTopLevelComments(commentIDs)

		// Confirm batch deletion or edit.
		if r.Method == http.MethodPost {

			confirm := r.PostFormValue("confirm") == "true"
			if !confirm {
				session.FlashError(w, r, "Confirm you want to modify these photos.")
				templates.Redirect(w, redirectURI)
				return
			}

			batchDeleteCommentPhotos(w, r, currentUser, photos, comments, threadMap)

			// Return the user to their gallery.
			templates.Redirect(w, redirectURI)
			return
		}

		var vars = map[string]interface{}{
			"Intent": intent,
			"Photos": photos,
		}

		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

// Batch DELETE executive handler.
func batchDeleteCommentPhotos(
	w http.ResponseWriter,
	r *http.Request,
	currentUser *models.User,
	photos map[uint64]*models.CommentPhoto,
	comments map[uint64]*models.Comment,
	threads map[uint64]*models.Thread,
) {
	// Delete all the photos.
	for _, photo := range photos {

		// Remove the image from disk.
		if err := pphoto.Delete(photo.Filename); err != nil {
			session.FlashError(w, r, "Delete Photo: couldn't remove file from disk: %s: %s", photo.Filename, err)
			return
		}

		// What do we do with CommentPhoto row?
		//
		// - If the attached Comment had text on it besides the photo, keep the comment.
		// - If the attached Comment is empty, delete it and the PhotoComment.
		var blankInsteadOfDelete bool
		if comment, ok := comments[photo.CommentID]; ok {

			// Empty comment apart from the picture: delete the comment as well.
			if strings.TrimSpace(comment.Message) == "" {
				if err := comment.Delete(); err != nil {
					session.FlashError(w, r, "Error deleting the empty comment message attached to the photo: %s", err)
				}
			} else {
				// The comment had text besides the picture: keep the PhotoComment but blank it out.
				blankInsteadOfDelete = true
			}
		}

		// If the comment is the top-level post of a thread, don't try and delete it (which would fail
		// due to foreign key constraint anyway), but treat it as a comment with text and just blank
		// out its attached photo.
		if _, ok := threads[photo.CommentID]; ok {
			blankInsteadOfDelete = true
		}

		// If the comment had a message besides just the picture, blank out the details
		// of this PhotoComment but keep it around for context.
		if blankInsteadOfDelete {
			// Update the CommentPhoto row: set its ExpiredAt and clear its filename.
			photo.Filename = ""
			photo.Filesize = 0
			photo.ExpiredAt = time.Now()

			if err := photo.Save(); err != nil {
				session.FlashError(w, r, "Couldn't update photo: %s", err)
				return
			}

			// Log the change.
			models.LogUpdated(currentUser, nil, "comment_photos", photo.ID, "The user removed this comment photo, but the attached comment (which had text besides) was retained.", nil)
		} else {
			// Delete the photo.
			if err := photo.Delete(); err != nil {
				session.FlashError(w, r, "Couldn't delete photo: %s", err)
				return
			}

			// Log the change.
			models.LogDeleted(currentUser, nil, "comment_photos", photo.ID, "Deleted the comment photo and its attached comment.", nil)
		}
	}

	session.Flash(w, r, "%d photo(s) deleted!", len(photos))
}
