package account

import (
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// EditEssays is a convenience page to edit the main profile textareas.
func EditEssays() http.HandlerFunc {
	tmpl := templates.Must("account/edit_essays.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Load the current user.
		var (
			currentUser, err = session.CurrentUser(r)
			user             = currentUser
		)
		if err != nil {
			session.FlashError(w, r, "Couldn't get CurrentUser: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Admin-only query param ?user_id=
		var userID = currentUser.ID
		if i, err := strconv.Atoi(r.FormValue("user_id")); err == nil {
			userID = uint64(i)
		}

		// If the current user is not admin, force the userID to themself.
		if currentUser.HasAdminScope(config.ScopePhotoModerator) && userID != currentUser.ID {
			if otherUser, err := models.GetUser(userID); err == nil {
				user = otherUser
			} else {
				session.FlashError(w, r, "User ID is not found.")
				templates.Redirect(w, "/")
				return
			}
		}

		// Are we POSTing?
		if r.Method == http.MethodPost {

			var (
				about       = r.PostFormValue("about_me")
				interests   = r.PostFormValue("interests")
				musicMovies = r.PostFormValue("music_movies")
			)

			// Set their Long profile fields.
			user.SetLongProfileField("about_me", about)
			user.SetLongProfileField("interests", interests)
			user.SetLongProfileField("music_movies", musicMovies)

			if err := user.Save(); err != nil {
				session.FlashError(w, r, "Error saving the user: %s", err)
			} else {
				session.Flash(w, r, "Their profile text has been updated!")
			}

			templates.Redirect(w, "/u/"+user.Username)
			return

		}

		vars := map[string]interface{}{
			"User": user,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
