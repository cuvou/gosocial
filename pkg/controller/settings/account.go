package settings

import (
	"fmt"
	"net/http"
	nm "net/mail"
	"strings"

	"github.com/cuvou/gosocial/pkg/chat"
	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/mail"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/redis"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/worker"
	"github.com/google/uuid"
)

// ChangeEmailToken for Redis.
type ChangeEmailToken struct {
	Token    string
	UserID   uint64
	NewEmail string
}

// Delete the change email token.
func (t ChangeEmailToken) Delete() error {
	return redis.Delete(fmt.Sprintf(config.ChangeEmailRedisKey, t.Token))
}

// Account settings page (/settings/account).
func Account() http.HandlerFunc {
	tmpl := templates.Must("settings/account.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Load the current user in case of updates.
		user, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get CurrentUser: %s", err)
			templates.Redirect(w, r.URL.Path)
			return
		}

		// Is the user currently in the chat room? Gate username changes when so.
		var isOnChat = worker.GetChatStatistics().IsOnline(user.Username)

		// Are we POSTing?
		if r.Method == http.MethodPost {

			var (
				oldPassword    = r.PostFormValue("old_password")
				changeEmail    = strings.TrimSpace(strings.ToLower(r.PostFormValue("change_email")))
				changeUsername = strings.TrimSpace(strings.ToLower(r.PostFormValue("change_username")))
				password1      = strings.TrimSpace(r.PostFormValue("new_password"))
				password2      = strings.TrimSpace(r.PostFormValue("new_password2"))
			)

			// Their old password is needed to make any changes to their account.
			if err := user.CheckPassword(oldPassword); err != nil {
				session.FlashError(w, r, "Could not make changes to your account settings as the 'current password' you entered was incorrect.")
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Changing their username?
			if changeUsername != user.Username {
				// Not if they are in the chat room!
				if isOnChat {
					session.FlashError(w, r, "Your username could not be changed right now because you are logged into the chat room. Please exit the chat room, wait a minute, and try your request again.")
					templates.Redirect(w, r.URL.Path)
					return
				}

				// Check if the new name is OK.
				if err := models.IsValidUsername(changeUsername); err != nil {
					session.FlashError(w, r, "Could not change your username: %s", err.Error())
					templates.Redirect(w, r.URL.Path)
					return
				}

				// Clear their history on the chat room.
				go func(username string) {
					log.Error("Change of username, clear chat history for old name %s", username)
					i, err := chat.EraseChatHistory(username)
					if err != nil {
						log.Error("EraseChatHistory(%s): %s", username, err)
						return
					}

					session.Flash(w, r, "Notice: due to your recent change in username, your direct message history on the Chat Room has been reset. %d message(s) had been removed.", i)
				}(user.Username)

				// Set their name.
				origUsername := user.Username
				user.Username = changeUsername
				if err := user.Save(); err != nil {
					session.FlashError(w, r, "Error saving your new username: %s", err)
				} else {
					session.Flash(w, r, "Your username has been updated to: %s", user.Username)

					// Notify the admin about this to keep tabs if someone is acting strangely
					// with too-frequent username changes.
					fb := &models.Feedback{
						Intent:    "report",
						Subject:   "Change of username",
						UserID:    user.ID,
						TableName: "users",
						TableID:   user.ID,
						Message: fmt.Sprintf(
							"A user has modified their username on their profile page!\n\n"+
								"* Original: %s\n* Updated: %s",
							origUsername, changeUsername,
						),
					}

					// Save the feedback.
					if err := models.CreateFeedback(fb); err != nil {
						log.Error("Couldn't save feedback from user updating their DOB: %s", err)
					}
				}
			}

			// Changing their email?
			if changeEmail != user.Email {
				// Validate the email.
				if _, err := nm.ParseAddress(changeEmail); err != nil {
					session.FlashError(w, r, "The email address you entered is not valid: %s", err)
					templates.Redirect(w, r.URL.Path)
					return
				}

				// Email must not already exist.
				if _, err := models.FindUsernameOrEmail(changeEmail); err == nil {
					session.FlashError(w, r, "That email address is already in use.")
					templates.Redirect(w, r.URL.Path)
					return
				}

				// Create a tokenized link.
				token := ChangeEmailToken{
					Token:    uuid.New().String(),
					UserID:   user.ID,
					NewEmail: changeEmail,
				}
				if err := redis.Set(fmt.Sprintf(config.ChangeEmailRedisKey, token.Token), token, config.SignupTokenExpires); err != nil {
					session.FlashError(w, r, "Failed to create change email token: %s", err)
					templates.Redirect(w, r.URL.Path)
					return
				}

				err := mail.Send(mail.Message{
					To:       changeEmail,
					Subject:  "Verify your e-mail address",
					Template: "email/verify_email.html",
					Data: map[string]interface{}{
						"Title":       config.Title,
						"URL":         config.Current.BaseURL + "/settings/confirm-email?token=" + token.Token,
						"ChangeEmail": true,
					},
				})
				if err != nil {
					session.FlashError(w, r, "Error sending a confirmation email to %s: %s", changeEmail, err)
				} else {
					session.Flash(w, r, "Please verify your new email address. A link has been sent to %s to confirm.", changeEmail)
				}
			}

			// Changing their password?
			if password1 != "" {
				if password2 != password1 {
					session.FlashError(w, r, "Couldn't change your password: your new passwords do not match.")
				} else {
					// Hash the new password.
					if err := user.HashPassword(password1); err != nil {
						session.FlashError(w, r, "Failed to hash your new password: %s", err)
					} else {
						// Save the user row.
						if err := user.Save(); err != nil {
							session.FlashError(w, r, "Failed to update your password in the database: %s", err)
						} else {
							session.Flash(w, r, "Your password has been updated.")
						}

						// Log out other sessions.
						session.LogoutOtherSessions(r)
					}
				}
			}

			templates.Redirect(w, r.URL.Path)
			return
		}

		vars := map[string]interface{}{
			"OnChat":           isOnChat,
			"TwoFactorEnabled": models.Get2FA(user.ID).Enabled,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

// ConfirmEmailChange after a user tries to change their email.
func ConfirmEmailChange() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var tokenStr = r.FormValue("token")

		if tokenStr != "" {
			var token ChangeEmailToken
			if err := redis.Get(fmt.Sprintf(config.ChangeEmailRedisKey, tokenStr), &token); err != nil {
				session.FlashError(w, r, "Invalid token. Please try again to change your email address.")
				templates.Redirect(w, "/")
				return
			}

			// Verify new email still doesn't already exist.
			if _, err := models.FindUsernameOrEmail(token.NewEmail); err == nil {
				session.FlashError(w, r, "Couldn't update your email address: it is already in use by another member.")
				templates.Redirect(w, "/")
				return
			}

			// Look up the user.
			user, err := models.GetUser(token.UserID)
			if err != nil {
				session.FlashError(w, r, "Didn't find the user that this email change was for. Please try again.")
				templates.Redirect(w, "/")
				return
			}

			// Burn the token.
			if err := token.Delete(); err != nil {
				log.Error("ChangeEmail: couldn't delete Redis token: %s", err)
			}

			// Make the change.
			user.Email = token.NewEmail
			if err := user.Save(); err != nil {
				session.FlashError(w, r, "Couldn't save the change to your user: %s", err)
			} else {
				session.Flash(w, r, "Your email address has been confirmed and updated.")
				templates.Redirect(w, "/")
			}
		} else {
			session.FlashError(w, r, "Invalid change email token. Please try again.")
		}

		templates.Redirect(w, "/")
	})
}
