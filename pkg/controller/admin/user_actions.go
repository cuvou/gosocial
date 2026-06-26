package admin

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cuvou/gosocial/pkg/chat"
	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/models/deletion"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Mark a user photo as Explicit for them.
func MarkPhotoExplicit() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			photoID uint64
			next    = r.FormValue("next")
		)

		if !strings.HasPrefix(next, "/") {
			next = "/"
		}

		// Get current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Failed to get current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		if idInt, err := strconv.Atoi(r.FormValue("photo_id")); err == nil {
			photoID = uint64(idInt)
		} else {
			session.FlashError(w, r, "Invalid or missing photo_id parameter: %s", err)
			templates.Redirect(w, next)
			return
		}

		// Get this photo.
		photo, err := models.GetPhoto(photoID)
		if err != nil {
			session.FlashError(w, r, "Didn't find photo ID in database: %s", err)
			templates.Redirect(w, next)
			return
		}

		photo.Explicit = true
		if err := photo.Save(); err != nil {
			session.FlashError(w, r, "Couldn't save photo: %s", err)
		} else {
			session.Flash(w, r, "Marked photo as Explicit!")
		}

		// Log the change.
		models.LogUpdated(&models.User{ID: photo.UserID}, currentUser, "photos", photo.ID, "Marked explicit by admin action.", []models.FieldDiff{
			models.NewFieldDiff("Explicit", false, true),
		})

		templates.Redirect(w, next)
	})
}

// Admin actions against a user account.
func UserActions() http.HandlerFunc {
	tmpl := templates.Must("admin/user_actions.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			intent  = r.FormValue("intent")
			confirm = r.Method == http.MethodPost
			reason  = r.FormValue("reason") // for impersonation
			userId  uint64
		)

		// Get current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Failed to get current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		if idInt, err := strconv.Atoi(r.FormValue("user_id")); err == nil {
			userId = uint64(idInt)
		} else {
			session.FlashError(w, r, "Invalid or missing user_id parameter: %s", err)
			templates.Redirect(w, "/admin")
			return
		}

		// Get this user.
		user, err := models.GetUser(userId)
		if err != nil {
			session.FlashError(w, r, "Didn't find user ID in database: %s", err)
			templates.Redirect(w, "/admin")
			return
		}

		// Template variables.
		var vars = map[string]interface{}{
			"Intent": intent,
			"User":   user,
		}

		switch intent {
		case "insights":
			// Admin insights (peek at block lists, etc.)
			if !currentUser.HasAdminScope(config.ScopeUserInsight) {
				session.FlashError(w, r, "Missing admin scope: %s", config.ScopeUserInsight)
				templates.Redirect(w, "/admin")
				return
			}

			// Get their block lists.
			insights, err := models.GetBlocklistInsights(user)
			if err != nil {
				session.FlashError(w, r, "Error getting blocklist insights: %s", err)
			}
			vars["BlocklistInsights"] = insights

			// Get their message insights.
			messages, err := models.GetMessageInsights(user)
			if err != nil {
				session.FlashError(w, r, "Error getting message insights: %s", err)
			}
			vars["MessageInsights"] = messages

			// Also surface counts of admin blocks.
			count, total := models.CountBlockedAdminUsers(user)
			vars["AdminBlockCount"] = count
			vars["AdminBlockTotal"] = total

			// Get their media usage stats.
			quota, err := models.GetUserMediaQuota(user)
			if err != nil {
				session.FlashError(w, r, "Error getting user media quota: %s", err)
			}
			vars["UserMediaQuota"] = quota

			// 2FA setting.
			vars["TwoFactorEnabled"] = models.Get2FA(user.ID).Enabled
		case "reset.2fa":
			// Reset 2FA for the user.
			if !currentUser.HasAdminScope(config.ScopeManage2FA) {
				session.FlashError(w, r, "Missing admin scope: %s", config.ScopeManage2FA)
				templates.Redirect(w, "/admin")
				return
			}

			if r.Method == http.MethodPost {
				var (
					confirm  = r.PostFormValue("confirm") == "true"
					confirm2 = r.PostFormValue("confirm2") == "confirm"
				)

				if !(confirm && confirm2) {
					session.FlashError(w, r, "Check the box and enter the word 'confirm' to disable 2FA for this account.")
				} else {
					// Disable it.
					tf := models.Get2FA(user.ID)
					if tf.Enabled {
						tf.Enabled = false
						if err := tf.Save(); err != nil {
							session.FlashError(w, r, "Error saving 2FA model: %s", err)
						}
					}
					session.Flash(w, r, "Two-Factor Auth has been deactivated on this account.")
					templates.Redirect(w, fmt.Sprintf("%s?intent=insights&user_id=%d", r.URL.Path, user.ID))
					return
				}

				templates.Redirect(w, fmt.Sprintf("%s?intent=reset.2fa&user_id=%d", r.URL.Path, user.ID))
				return
			}
		case "impersonate":
			// Scope check.
			if !currentUser.HasAdminScope(config.ScopeUserImpersonate) {
				session.FlashError(w, r, "Missing admin scope: %s", config.ScopeUserImpersonate)
				templates.Redirect(w, "/admin")
				return
			}

			if confirm {
				if err := session.ImpersonateUser(w, r, user, currentUser, reason); err != nil {
					session.FlashError(w, r, "Failed to impersonate user: %s", err)
				} else {
					session.Flash(w, r, "You are now impersonating %s", user.Username)
					templates.Redirect(w, "/me")
					return
				}
			}
		case "ban":
			// Scope check.
			if !currentUser.HasAdminScope(config.ScopeUserBan) {
				session.FlashError(w, r, "Missing admin scope: %s", config.ScopeUserBan)
				templates.Redirect(w, "/admin")
				return
			}

			if confirm {
				status := r.PostFormValue("status")

				switch status {
				case "active":
					user.Status = models.UserStatusActive
				case "banned":
					user.Status = models.UserStatusBanned
				case "disabled":
					user.Status = models.UserStatusDisabled
				default:
					session.FlashError(w, r, "Unexpected user status: %s", status)
				}

				user.Save()
				session.Flash(w, r, "User ban status updated!")
				templates.Redirect(w, "/u/"+user.Username)

				// Maybe kick them from chat room now.
				if _, err := chat.MaybeDisconnectUser(user); err != nil {
					log.Error("chat.MaybeDisconnectUser(%s#%d): %s", user.Username, user.ID, err)
				}

				// Log the change.
				models.LogEvent(user, currentUser, models.ChangeLogBanned, "users", currentUser.ID, fmt.Sprintf("User ban status updated to: %s", status))
				return
			}
		case "promote":
			// Scope check.
			if !currentUser.HasAdminScope(config.ScopeUserPromote) {
				session.FlashError(w, r, "Missing admin scope: %s", config.ScopeUserPromote)
				templates.Redirect(w, "/admin")
				return
			}

			if confirm {
				action := r.PostFormValue("action")
				user.IsAdmin = action == "promote"
				user.Save()
				session.Flash(w, r, "User admin status updated!")
				templates.Redirect(w, "/u/"+user.Username)

				// Log the change.
				models.LogEvent(user, currentUser, models.ChangeLogAdmin, "users", currentUser.ID, fmt.Sprintf("User admin status updated to: %s", action))
				return
			}
		case "password":
			// Scope check.
			if !currentUser.HasAdminScope(config.ScopeUserPassword) {
				session.FlashError(w, r, "Missing admin scope: %s", config.ScopeUserPassword)
				templates.Redirect(w, "/admin")
				return
			}

			if confirm {
				password := strings.TrimSpace(r.PostFormValue("password"))
				if len(password) < 3 {
					session.FlashError(w, r, "A password of at least 3 characters is required.")
					templates.Redirect(w, r.URL.Path+fmt.Sprintf("?intent=password&user_id=%d", user.ID))
					return
				}

				if err := user.SaveNewPassword(password); err != nil {
					session.FlashError(w, r, "Failed to set the user's password: %s", err)
				} else {
					session.Flash(w, r, "The user's password has been updated to: %s", password)
				}

				// Log out all of the user's sessions.
				models.RevokeAllUserLogins(user.ID)

				templates.Redirect(w, "/u/"+user.Username)
				return
			}
		case "delete":
			// Scope check.
			if !currentUser.HasAdminScope(config.ScopeUserDelete) {
				session.FlashError(w, r, "Missing admin scope: %s", config.ScopeUserDelete)
				templates.Redirect(w, "/admin")
				return
			}

			if confirm {

				if err := deletion.DeleteUser(user); err != nil {
					session.FlashError(w, r, "Failed when deleting the user: %s", err)
				} else {
					session.Flash(w, r, "User has been deleted!")
				}
				templates.Redirect(w, "/admin")

				// Kick them from the chat room if they are online.
				if _, err := chat.DisconnectUserNow(user, "You have been signed out of chat because your account has been deleted."); err != nil {
					log.Error("chat.MaybeDisconnectUser(%s#%d): %s", user.Username, user.ID, err)
				}

				// Log the change.
				models.LogDeleted(nil, currentUser, "users", user.ID, fmt.Sprintf("Username %s has been deleted by an admin.", user.Username), nil)
				return
			}
		default:
			session.FlashError(w, r, "Unsupported admin user intent: %s", intent)
			templates.Redirect(w, "/admin")
			return
		}

		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

// Un-impersonate a user account.
func Unimpersonate() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sess := session.Get(r)
		if sess.Impersonator > 0 {
			user, err := models.GetUser(sess.Impersonator)
			if err != nil {
				session.FlashError(w, r, "Couldn't unimpersonate: impersonator (%d) is not an admin!", user.ID)
				templates.Redirect(w, "/")
				return
			}

			session.LoginUser(w, r, user)
			session.Flash(w, r, "No longer impersonating.")
			templates.Redirect(w, "/")
		}
		templates.Redirect(w, "/")
	})
}
