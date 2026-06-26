package photo

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/chat"
	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	pphoto "github.com/cuvou/gosocial/pkg/photo"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// BatchEdit controller (/photo/batch-edit?id=N) to change properties about your picture.
func BatchEdit() http.HandlerFunc {
	tmpl := templates.Must("photo/batch_edit.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			// Form params
			intent   = r.FormValue("intent")
			photoIDs []uint64
		)

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
		photos, err := models.GetPhotos(photoIDs)
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
		var (
			ownerIDs    []uint64
			allMyPhotos = true // all photos belong to the current user
		)
		for _, photo := range photos {

			if !photo.CanBeEditedBy(currentUser) {
				templates.ForbiddenPage(w, r)
				return
			}

			ownerIDs = append(ownerIDs, photo.UserID)

			if photo.UserID != currentUser.ID {
				allMyPhotos = false
			}
		}

		// Deletion safety with dedicated Admin scope.
		if intent == "delete" && !allMyPhotos && !currentUser.HasAdminScope(config.ScopePhotoModerator) {
			session.FlashError(w, r, "Missing required admin scope: %s", config.ScopePhotoModerator)
			templates.ForbiddenPage(w, r)
			return
		}

		// Load the photo owners.
		var (
			owners, _   = models.MapUsers(currentUser, ownerIDs)
			redirectURI = "/" // go first owner's gallery
		)
		for _, user := range owners {
			redirectURI = fmt.Sprintf("/u/%s/photos", user.Username)
		}

		// Confirm batch deletion or edit.
		if r.Method == http.MethodPost {

			confirm := r.PostFormValue("confirm") == "true"
			if !confirm {
				session.FlashError(w, r, "Confirm you want to modify this photo.")
				templates.Redirect(w, redirectURI)
				return
			}

			// Which intent are they executing on?
			switch intent {
			case "delete":
				batchDeletePhotos(w, r, currentUser, photos, owners, redirectURI)
			case "visibility":
				batchUpdateVisibility(w, r, currentUser, photos, owners)
			default:
				session.FlashError(w, r, "Unknown intent")
			}

			// If the user is currently on chat, push their updated JWT token
			// in case we need to update their Shy Account rules.
			go func() {
				if err := chat.AmendJWTToken(r, currentUser.ID); err != nil {
					log.Error("AmendJWTToken: Couldn't send amended JWT token for %s to chat room: %s", currentUser.Username, err)
				}
			}()

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
func batchDeletePhotos(
	w http.ResponseWriter,
	r *http.Request,
	currentUser *models.User,
	photos map[uint64]*models.Photo,
	owners map[uint64]*models.User,
	redirectURI string,
) {
	// Delete all the photos.
	for _, photo := range photos {

		// Remove the images from disk.
		for _, filename := range []string{
			photo.Filename,
			photo.CroppedFilename,
		} {
			if len(filename) > 0 {
				if err := pphoto.Delete(filename); err != nil {
					session.FlashError(w, r, "Delete Photo: couldn't remove file from disk: %s: %s", filename, err)
				}
			}
		}

		// Take back notifications on it.
		models.RemoveNotification("photos", photo.ID)

		if err := photo.Delete(); err != nil {
			session.FlashError(w, r, "Couldn't delete photo: %s", err)
			templates.Redirect(w, redirectURI)
			return
		}

		// Log the change.
		if owner, ok := owners[photo.UserID]; ok {
			models.LogDeleted(owner, currentUser, "photos", photo.ID, "Deleted the photo.", photo)
		}
	}

	session.Flash(w, r, "%d photo(s) deleted!", len(photos))
}

// Batch DELETE executive handler.
func batchUpdateVisibility(
	w http.ResponseWriter,
	r *http.Request,
	currentUser *models.User,
	photos map[uint64]*models.Photo,
	owners map[uint64]*models.User,
) {
	// Visibility setting.
	visibility := r.PostFormValue("visibility")

	// Delete all the photos.
	for _, photo := range photos {

		// Diff for the ChangeLog.
		diffs := []models.FieldDiff{
			models.NewFieldDiff("Visibility", photo.Visibility, visibility),
		}

		photo.Visibility = models.PhotoVisibility(visibility)

		// If going private, take back notifications on it.
		if photo.Visibility == models.PhotoPrivate {
			models.RemoveNotification("photos", photo.ID)
		}

		if err := photo.Save(); err != nil {
			session.FlashError(w, r, "Error saving photo #%d: %s", photo.ID, err)
		}

		// Log the change.
		if owner, ok := owners[photo.UserID]; ok {
			// Log the change.
			models.LogUpdated(owner, currentUser, "photos", photo.ID, "Updated the photo's settings.", diffs)
		}
	}

	session.Flash(w, r, "%d photo(s) updated!", len(photos))
}
