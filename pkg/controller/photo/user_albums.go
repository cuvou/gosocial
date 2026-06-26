package photo

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// UserAlbums controller (/u/:username/albums) to list a user's albums.
//
// It is a stripped-down Gallery page with no photos but more albums.
func UserAlbums() http.HandlerFunc {
	tmpl := templates.Must("photo/gallery.html")

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Query params.
		var (
			username = r.PathValue("username")
		)

		// Find this user.
		user, err := models.FindUsername(username)
		if err != nil {
			templates.NotFoundPage(w, r)
			return
		}

		// Load the current user in case they are viewing their own page.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: couldn't get CurrentUser")
		}

		var (
			isOwnPhotos = currentUser.ID == user.ID
			areFriends  = isOwnPhotos || models.AreFriends(user.ID, currentUser.ID)
			isPrivate   = user.Visibility == models.UserVisibilityPrivate && !areFriends
		)

		// Is either one blocking?
		if err := user.CanBeSeenBy(currentUser); err != nil {
			templates.NotFoundPage(w, r)
			return
		}

		// Is this user private and we're not friends?
		if isPrivate && !currentUser.IsAdmin && !isOwnPhotos {
			session.FlashError(w, r, "This user's profile page and photo gallery are private.")
			templates.Redirect(w, "/u/"+user.Username)
			return
		}

		// Profile tab counts.
		tabCounts, err := user.GetProfileTabCounts(currentUser)
		if err != nil {
			session.FlashError(w, r, "Error getting profile tab counts: %s", err)
		}

		// Set user relationship booleans.
		if err := models.SetUserRelationships(currentUser, []*models.User{user}); err != nil {
			session.FlashError(w, r, "Error setting user relationships: %s", err)
		}

		var vars = map[string]interface{}{
			"ActiveProfileTab": "photos",
			"IsOwnPhotos":      currentUser.ID == user.ID,
			"User":             user,

			// Profile tabs.
			"ProfileTabCount": tabCounts,

			// Photo albums.
			"IsAlbumListView": true,
		}

		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
