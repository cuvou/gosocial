package friend

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/webpush"
)

// AddFriend controller to send a friend request.
func AddFriend() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// POST only.
		if r.Method != http.MethodPost {
			session.FlashError(w, r, "Unacceptable Request Method")
			templates.Redirect(w, "/")
			return
		}

		// Form fields
		var (
			username = strings.ToLower(r.PostFormValue("username"))
			verdict  = r.PostFormValue("verdict")
			message  = strings.TrimSpace(r.PostFormValue("message"))
		)

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get CurrentUser: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Get the target user.
		user, err := models.FindUsername(username)
		if err != nil {
			session.FlashError(w, r, "User Not Found")
			templates.Redirect(w, "/")
			return
		}

		// Can't friend yourself.
		if currentUser.ID == user.ID {
			session.FlashError(w, r, "You can't send a friend request to yourself!")
			templates.Redirect(w, "/u/"+username)
			return
		}

		// Is there a pending friend request from this user?
		// e.g., if you "Approve" a friend req by visiting their profile page and clicking Add Friend,
		// instead of approving it from your Requests page.
		hadRequestFrom := models.FriendStatus(currentUser.ID, user.ID) == "requested"

		// Are we adding, or rejecting+removing?
		switch verdict {
		case "reject", "remove":
			err := models.RemoveFriend(currentUser.ID, user.ID)
			if err != nil {
				session.FlashError(w, r, "Failed to remove friend: %s", err)
				templates.Redirect(w, "/u/"+username)
				return
			}

			// Revoke any friends-only photo notifications they had received before.
			if err := models.RevokeFriendPhotoNotifications(currentUser, user); err != nil {
				log.Error("Couldn't revoke friend photo notifications between %s and %s: %s", currentUser.Username, user.Username, err)
			}

			var message string
			if verdict == "reject" {
				message = fmt.Sprintf("Friend request from %s has been rejected.", username)
			} else {
				message = fmt.Sprintf("Removed friendship with %s.", username)
			}

			session.Flash(w, r, message)
			if verdict == "reject" {
				templates.Redirect(w, "/friends?view=requests")

				// Log the change.
				models.LogDeleted(currentUser, nil, "friends", user.ID, "Rejected friend request from "+user.Username+".", nil)
			} else {
				// Log the change.
				models.LogDeleted(currentUser, nil, "friends", user.ID, "Removed friendship with "+user.Username+".", nil)
			}
			templates.Redirect(w, "/friends")
			return
		case "ignore":
			if err := models.IgnoreFriendRequest(currentUser, user); err != nil {
				session.FlashError(w, r, "Error marking the friend request as ignored: %s", err)
			} else {
				session.Flash(w, r, "You have ignored the friend request from %s.", username)
			}
			templates.Redirect(w, "/friends")

			// Log the change.
			models.LogUpdated(currentUser, nil, "friends", user.ID, "Ignored the friend request from "+user.Username+".", nil)
			return
		default:
			// Post the friend request.
			if err := models.AddFriend(currentUser.ID, user.ID, message); err != nil {
				session.FlashError(w, r, "Couldn't send friend request: %s.", err)
			} else {
				if verdict == "approve" || hadRequestFrom {
					// Notify the requestor they'd been approved.
					if !user.NotificationOptOut(config.NotificationOptOutFriendRequestAccepted) {
						notif := &models.Notification{
							UserID:    user.ID,
							AboutUser: *currentUser,
							Type:      models.NotificationFriendApproved,
						}
						if err := models.CreateNotification(notif); err != nil {
							log.Error("Couldn't create approved notification: %s", err)
						}
					}

					session.Flash(w, r, "You accepted the friend request from %s!", username)
					templates.Redirect(w, "/friends?view=requests")

					// Follow the user back.
					if _, err := models.AddFollow(currentUser.ID, user.ID); err != nil {
						session.FlashError(w, r, "Error following the user: %s", err)
					}

					// If we (currentUser) had friends-only followers enabled, so that the requestor
					// couldn't follow us up front; then add the reverse Follow now as well.
					doFollow := models.GetPrivacySetting(currentUser.ID).FollowMe == "friends"
					if doFollow {
						if _, err := models.AddFollow(user.ID, currentUser.ID); err != nil {
							session.FlashError(w, r, "Error following the user back: %s", err)
						}
					}

					// Log the change.
					models.LogUpdated(currentUser, nil, "friends", user.ID, "Accepted friend request from "+user.Username+".", nil)
					return
				} else {
					// Log the change.
					models.LogCreated(currentUser, "friends", user.ID, "Sent a friend request to "+user.Username+".")

					// Follow the user immediately (if allowed).
					doFollow := models.GetPrivacySetting(user.ID).FollowMe != "friends"
					if doFollow {
						if _, err := models.AddFollow(currentUser.ID, user.ID); err != nil {
							session.FlashError(w, r, "Error following the user: %s", err)
						} else {
							// Notify the recipient of the follow.
							notif := &models.Notification{
								UserID:      user.ID,
								AboutUserID: &currentUser.ID,
								Type:        models.NotificationFollow,
								TableName:   "users",
								TableID:     currentUser.ID,
							}
							if err := models.CreateNotification(notif); err != nil {
								log.Error("Couldn't notify user %s about being followed: %s", user.ID, err)
							}
						}
					}

					// Send a push notification to the recipient.
					go func() {
						// Opted out of this one?
						if user.GetProfileField(config.PushNotificationOptOutFriends) == "true" {
							return
						}

						log.Info("Try and send Web Push notification about new Friend Request to: %s", user.Username)
						webpush.SendNotification(user, webpush.Payload{
							Topic: "friend",
							Title: "New Friend Request!",
							Body:  fmt.Sprintf("%s wants to be your friend on %s.", currentUser.Username, config.Title),
						})
					}()
				}
				session.Flash(w, r, "Friend request sent!")
			}
		}

		templates.Redirect(w, "/u/"+username)
	})
}
