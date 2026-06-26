package settings

import (
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/oklog/ulid/v2"
)

// LoginSessions settings (/settings/sessions).
func LoginSessions() http.HandlerFunc {
	tmpl := templates.Must("settings/login_sessions.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			page, _ = strconv.Atoi(r.FormValue("page"))
		)

		if page < 1 {
			page = 1
		}

		// Load the current user in case of updates.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get CurrentUser: %s", err)
			templates.Redirect(w, r.URL.Path)
			return
		}

		// Get our current Session ID to match with the list.
		sessionID := session.Get(r).ULID

		// Are we POSTing?
		if r.Method == http.MethodPost {
			var (
				intent   = r.PostFormValue("intent")
				revokeID = r.PostFormValue("id")
			)

			switch intent {
			case "revoke-all":
				if err := models.RevokeAllLoginSessions(currentUser, sessionID); err != nil {
					session.FlashError(w, r, "Error revoking login sessions: %s", err)
				} else {
					session.Flash(w, r, "All other devices have been logged out successfully.")
				}
			case "revoke":
				// Validate the ULID.
				if id, err := ulid.Parse(revokeID); err != nil {
					session.FlashError(w, r, "Invalid session number: %s", err)
				} else {
					if err := models.RevokeLoginSession(id); err != nil {
						session.FlashError(w, r, "Error revoking the login session: %s", err)
					} else {
						session.Flash(w, r, "That device has now been logged out of your account.")
					}
				}
			}

			templates.Redirect(w, r.URL.Path)
			return
		}

		// Get their login sessions.
		var pager = &models.Pagination{
			Page:    page,
			PerPage: config.PageSizeLoginSessions,
			Sort:    "updated_at desc",
		}
		ls, err := models.PaginateLoginSessions(currentUser, pager)
		if err != nil {
			session.FlashError(w, r, "Error listing your login sessions: %s", err)
		}

		vars := map[string]interface{}{
			"LoginSessions": ls,
			"Pager":         pager,
			"MySessionID":   sessionID.String(),
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
