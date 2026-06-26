package api

import (
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
)

// ViewPhoto API pings a view count on a photo, e.g. from the lightbox modal.
func ViewPhoto() http.HandlerFunc {
	// Response JSON schema.
	type Response struct {
		OK    bool   `json:"OK"`
		Error string `json:"error,omitempty"`
		Likes int64  `json:"likes"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			SendJSON(w, http.StatusBadRequest, Response{
				Error: "Couldn't get current user!",
			})
			return
		}

		// Photo ID from path parameter.
		var photoID uint64
		if id, err := strconv.Atoi(r.PathValue("photo_id")); err == nil && id > 0 {
			photoID = uint64(id)
		} else {
			SendJSON(w, http.StatusBadRequest, Response{
				Error: "Invalid photo ID",
			})
			return
		}

		// Find this photo.
		photo, err := models.GetPhoto(photoID)
		if err != nil {
			SendJSON(w, http.StatusNotFound, Response{
				Error: "Photo Not Found",
			})
			return
		}

		// Check permission to have seen this photo.
		if ok, err := photo.ShouldBeSeenBy(currentUser); !ok {
			log.Error("Photo %d can't be seen by %s: %s", photo.ID, currentUser.Username, err)
			SendJSON(w, http.StatusNotFound, Response{
				Error: "Photo Not Found",
			})
			return
		}

		// Mark a view.
		if err := photo.View(currentUser); err != nil {
			log.Error("Update photo(%d) views: %s", photo.ID, err)
		}

		// Send success response.
		SendJSON(w, http.StatusOK, Response{
			OK: true,
		})
	})
}
