package comment

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/markdown"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// PostComment view - for previewing or submitting your comment.
func PostComment() http.HandlerFunc {
	tmpl := templates.Must("comment/post_comment.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Query params.
		var (
			tableName     = r.FormValue("table_name")
			tableID       uint64
			editCommentID = r.FormValue("edit") // edit your comment
			isDelete      = r.FormValue("delete") == "true"
			intent        = r.FormValue("intent")      // preview or submit
			message       = r.PostFormValue("message") // comment body
			comment       *models.Comment              // if editing a comment
			fromURL       = r.FormValue("next")        // what page to send back to

			// Should the new comment be forbidden? (e.g. !photo.ShouldBeSeenBy)
			forbidden         bool
			contentVisibility string
		)

		// Parse the table ID param.
		if idStr := r.FormValue("table_id"); idStr == "" {
			session.FlashError(w, r, "Comment table ID required.")
			templates.Redirect(w, "/")
			return
		} else {
			if idInt, err := strconv.Atoi(idStr); err != nil {
				session.FlashError(w, r, "Comment table ID invalid.")
				templates.Redirect(w, "/")
				return
			} else {
				tableID = uint64(idInt)
			}
		}

		// Redirect URL must be relative.
		if !strings.HasPrefix(fromURL, "/") {
			// Maybe it's URL encoded?
			fromURL, _ = url.QueryUnescape(fromURL)
			if !strings.HasPrefix(fromURL, "/") {
				fromURL = "/"
			}
		}

		// Validate everything else.
		if _, ok := models.CommentableTables[tableName]; !ok {
			session.FlashError(w, r, "You can not comment on that.")
			templates.Redirect(w, "/")
			return
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Who will we notify about this comment? e.g. if commenting on a photo,
		// this is the user who owns the photo.
		var notifyUser *models.User
		switch tableName {
		case "photos":
			if photo, err := models.GetPhoto(tableID); err == nil {
				if user, err := models.GetUser(photo.UserID); err == nil {
					notifyUser = user

					// Safety check: should the current user be able to see this photo?
					if ok, _ := photo.ShouldBeSeenBy(currentUser); !ok {
						forbidden = true
						contentVisibility = string(photo.Visibility)
					}
				} else {
					log.Error("Comments: couldn't get NotifyUser for photo ID %d (user ID %d): %s",
						tableID, photo.UserID, err,
					)
				}
			} else {
				log.Error("Comments: couldn't get NotifyUser for photo ID %d: %s", tableID, err)
			}
		}

		// Check permission to comment in case it's a photo and the owner restricts who can comment.
		if tableName == "photos" && notifyUser != nil {
			privacySetting := models.GetPrivacySetting(notifyUser.ID)
			permission := privacySetting.PhotoComments
			if permission == "nobody" || (permission == "friends" && !models.AreFriends(currentUser.ID, notifyUser.ID)) {
				session.FlashError(w, r, "The owner of that photo is not accepting any new comments to be added.")
				templates.Redirect(w, fromURL)
				return
			}
		}

		// Should adding this comment be forbidden?
		if forbidden {
			// If the current user is not an admin, bail now.
			if !currentUser.IsAdmin {
				templates.ForbiddenPage(w, r)
				return
			}

			// Log an admin report for visibility.
			aboutUsername := "[unavailable]"
			if notifyUser != nil {
				aboutUsername = notifyUser.Username
			}
			fb := &models.Feedback{
				Intent:    "report",
				Subject:   "Admin Comment on Private Content",
				UserID:    currentUser.ID,
				TableName: tableName,
				TableID:   tableID,
				Message: fmt.Sprintf(
					"The admin user **@%s** has left a comment on **@%s**'s private content.\n\n"+
						"* Table: %s#%d\n* Owner: [@%s](/u/%s)\n* Content's visibility: %s\n\nTheir comment was:\n\n%s",
					currentUser.Username,
					aboutUsername,
					tableName,
					tableID,
					aboutUsername,
					aboutUsername,
					contentVisibility,
					markdown.Quotify(message),
				),
			}
			if err := models.CreateFeedback(fb); err != nil {
				log.Error("NotifyAboutModerationRules: error saving admin report: %s", err)
			}
		}

		// Are we editing or deleting our comment?
		if len(editCommentID) > 0 {
			if i, err := strconv.Atoi(editCommentID); err == nil {
				if found, err := models.GetComment(uint64(i)); err == nil {
					comment = found

					// Verify that it is indeed OUR comment to manage:
					// - If the current user posted it
					// - If we are an admin
					// - If we are the notifyUser for this comment (they can delete, not edit).
					if currentUser.ID != comment.UserID && !currentUser.IsAdmin &&
						!(notifyUser != nil && currentUser.ID == notifyUser.ID && isDelete) {
						templates.ForbiddenPage(w, r)
						return
					}

					// Initialize the form w/ the content of this message.
					if r.Method == http.MethodGet {
						message = comment.Message
					}

					// Are we DELETING this comment?
					if isDelete {
						// Revoke notifications.
						models.RemoveCommentNotification(comment)

						if err := comment.Delete(); err != nil {
							session.FlashError(w, r, "Error deleting your commenting: %s", err)
						} else {
							session.Flash(w, r, "Your comment has been deleted.")

							// Log the change.
							models.LogDeleted(&models.User{ID: comment.UserID}, currentUser, "comments", comment.ID, "Deleted a comment.", comment)
						}

						// Refresh cached like counts.
						switch tableName {
						case "photos":
							if err := models.UpdatePhotoCachedCounts(tableID); err != nil {
								log.Error("UpdatePhotoCachedCount(%d): %s", tableID, err)
							}
						}

						templates.Redirect(w, fromURL)
						return
					}
				} else {
					// Comment not found - show the Forbidden page anyway.
					templates.ForbiddenPage(w, r)
					return
				}
			} else {
				templates.NotFoundPage(w, r)
				return
			}
		}

		// Submitting the form.
		if r.Method == http.MethodPost {
			// Default intent is preview unless told to submit.
			if intent == "submit" {
				// A message is required.
				if message == "" {
					session.FlashError(w, r, "A message is required for your comment.")
					templates.Redirect(w, fromURL)
					return
				}

				// Are we modifying an existing comment?
				if comment != nil {
					comment.Message = message

					if err := comment.Save(); err != nil {
						session.FlashError(w, r, "Couldn't save comment: %s", err)
					} else {
						session.Flash(w, r, "Comment updated!")

						// Log the change.
						models.LogUpdated(&models.User{ID: comment.UserID}, currentUser, "comments", comment.ID, "Updated a comment.\n\n---\n\n"+comment.Message, nil)
					}
					templates.Redirect(w, fromURL)
					return
				}

				// Create the comment.
				if comment, err := models.AddComment(
					currentUser,
					tableName,
					tableID,
					message,
				); err != nil {
					session.FlashError(w, r, "Couldn't create comment: %s", err)
				} else {
					session.Flash(w, r, "Comment added!")
					templates.Redirect(w, fromURL)

					// Refresh cached comment counts.
					switch tableName {
					case "photos":
						if err := models.UpdatePhotoCachedCounts(tableID); err != nil {
							log.Error("UpdatePhotoCachedCount(%d): %s", tableID, err)
						}
					}

					// Log the change.
					models.LogCreated(currentUser, "comments", comment.ID, "Posted a new comment.\n\n---\n\n"+message)

					// Notify the recipient of the comment.
					if notifyUser != nil && notifyUser.ID != currentUser.ID && !notifyUser.NotificationOptOut(config.NotificationOptOutComments) {
						notif := &models.Notification{
							UserID:    notifyUser.ID,
							AboutUser: *currentUser,
							Type:      models.NotificationComment,
							TableName: comment.TableName,
							TableID:   comment.TableID,
							Message:   message,
							Link:      fmt.Sprintf("/go/comment?id=%d", comment.ID),
						}
						if err := models.CreateNotification(notif); err != nil {
							log.Error("Couldn't create Comment notification: %s", err)
						}
					}

					// Notify subscribers to this comment thread (filter the subscribers by the blocking status of the current user).
					for _, userID := range models.FilterBlockingUserIDs(currentUser, models.GetSubscribers(comment.TableName, comment.TableID)) {
						if notifyUser != nil && userID == notifyUser.ID {
							// Don't notify the recipient twice.
							continue
						} else if userID == currentUser.ID {
							// Don't notify the poster of the comment.
							continue
						}

						notif := &models.Notification{
							UserID:    userID,
							AboutUser: *currentUser,
							Type:      models.NotificationAlsoCommented,
							TableName: comment.TableName,
							TableID:   comment.TableID,
							Message:   message,
							Link:      fmt.Sprintf("/go/comment?id=%d", comment.ID),
						}
						if err := models.CreateNotification(notif); err != nil {
							log.Error("Couldn't create Comment notification for subscriber %d: %s", userID, err)
						}
					}

					// Subscribe the current user to this comment thread, so they are
					// notified if other users add followup comments.
					if !currentUser.NotificationOptOut(config.NotificationOptOutSubscriptions) {
						if _, err := models.SubscribeTo(currentUser, comment.TableName, comment.TableID); err != nil {
							log.Error("Couldn't subscribe user %d to comment thread %s/%d: %s",
								currentUser.ID, comment.TableName, comment.TableID, err,
							)
						}
					}

					return
				}
			}
		}

		var vars = map[string]interface{}{
			"Intent":        intent,
			"EditCommentID": editCommentID,
			"Message":       message,
			"TableName":     tableName,
			"TableID":       tableID,
			"Next":          fromURL,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
