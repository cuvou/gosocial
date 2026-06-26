package photo

import (
	"fmt"
	"net/http"

	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// GoToProfilePhoto redirects to a user's default profile pic, or gallery if not visible.
func GoToProfilePhoto() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the username out of the URL parameters.
		var username = r.PathValue("username")

		// Find this user.
		user, err := models.FindUsername(username)
		if err != nil {
			templates.NotFoundPage(w, r)
			return
		}

		// Get the viewer.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: could not get currentUser.")
			templates.Redirect(w, "/")
			return
		}

		if ok, _ := user.CanSeeProfilePicture(currentUser); ok && user.ProfilePhoto.ID > 0 {
			templates.Redirect(w, fmt.Sprintf("/photo/view?id=%d", user.ProfilePhoto.ID))
		} else {
			templates.Redirect(w, fmt.Sprintf("/u/%s/photos", user.Username))
		}
	})
}
