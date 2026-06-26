package admin

import (
	"net/http"
	"net/mail"
	"strings"

	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Manually create new user accounts.
func AddUser() http.HandlerFunc {
	tmpl := templates.Must("admin/add_user.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			var (
				email    = strings.TrimSpace(strings.ToLower(r.PostFormValue("email")))
				username = strings.TrimSpace(strings.ToLower(r.PostFormValue("username")))
				password = r.PostFormValue("password")
			)

			// Validate the email.
			if _, err := mail.ParseAddress(email); err != nil {
				session.FlashError(w, r, "The email address you entered is not valid: %s", err)
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Password check.
			if len(password) < 3 {
				session.FlashError(w, r, "The password is required to be 3+ characters long.")
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Validate the username is OK: well formatted, not reserved, not existing.
			if err := models.IsValidUsername(username); err != nil {
				session.FlashError(w, r, err.Error())
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Create the user.
			if _, err := models.CreateUser(username, email, password); err != nil {
				session.FlashError(w, r, "Couldn't create the user: %s", err)
				templates.Redirect(w, r.URL.Path)
				return
			}

			session.Flash(w, r, "Created the username %s with password: %s", username, password)
			templates.Redirect(w, r.URL.Path)
			return
		}

		if err := tmpl.Execute(w, r, nil); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
