package forum

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Delete page.
func Delete() http.HandlerFunc {
	tmpl := templates.Must("forum/delete.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			editStr = r.FormValue("id")
			editID  uint64

			// Confirmation fields.
			fragment = r.PostFormValue("fragment")
			intent   = r.PostFormValue("intent")
		)

		if i, err := strconv.Atoi(editStr); err == nil {
			editID = uint64(i)
		} else {
			session.FlashError(w, r, "Edit parameter: id was not an integer")
			templates.Redirect(w, "/forum/admin")
			return
		}

		// Redirects
		var (
			errorURL   = fmt.Sprintf("/forum/admin/delete?id=%d", editID)
			successURL = "/forum/admin"
		)

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// If editing, look up the existing forum.
		forum, err := models.GetForum(editID)
		if err != nil {
			session.FlashError(w, r, "Couldn't get forum: %s", err)
			templates.Redirect(w, successURL)
			return
		} else {
			// Do we have permission?
			if !forum.CanEdit(currentUser) {
				templates.ForbiddenPage(w, r)
				return
			}
		}

		// Saving?
		if r.Method == http.MethodPost {
			if intent != "confirm" {
				session.FlashError(w, r, "Unexpected intent: %s", intent)
				templates.Redirect(w, errorURL)
				return
			}

			// Validate the fragment for confirmation.
			if fragment != forum.Fragment {
				session.FlashError(w, r, "You must enter the forum's URL fragment to confirm deletion.")
				templates.Redirect(w, errorURL)
				return
			}

			if err := forum.Delete(); err != nil {
				session.FlashError(w, r, "Error deleting the forum: %s", err)
				templates.Redirect(w, errorURL)
				return
			}

			session.Flash(w, r, "Forum has been deleted!")
			templates.Redirect(w, successURL)
			return
		}

		var vars = map[string]interface{}{
			"Forum": forum,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
