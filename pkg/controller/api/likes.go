package api

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/photo"
	"github.com/cuvou/gosocial/pkg/session"
)

// Likes API posts a new like on something.
func Likes() http.HandlerFunc {
	// Request JSON schema.
	type Request struct {
		TableName string `json:"name"`
		TableID   string `json:"id"`
		Unlike    bool   `json:"unlike,omitempty"`
		Referrer  string `json:"page"`
	}

	// Response JSON schema.
	type Response struct {
		OK    bool   `json:"OK"`
		Error string `json:"error,omitempty"`
		Likes int64  `json:"likes"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			SendJSON(w, http.StatusNotAcceptable, Response{
				Error: "POST method only",
			})
			return
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			SendJSON(w, http.StatusBadRequest, Response{
				Error: "Couldn't get current user!",
			})
			return
		}

		// Parse request payload.
		var req Request
		if err := ParseJSON(r, &req); err != nil {
			SendJSON(w, http.StatusBadRequest, Response{
				Error: fmt.Sprintf("Error with request payload: %s", err),
			})
			return
		}

		// Sanity check things. The page= param (Referrer) must be a relative URL, the path
		// is useful for "liked your comment" notifications to supply the Link URL for the
		// notification.
		if len(req.Referrer) > 0 && req.Referrer[0] != '/' {
			req.Referrer = ""
		}

		// The link to attach to the notification.
		var linkTo = req.Referrer

		// Is the ID an integer?
		var tableID uint64
		if v, err := strconv.Atoi(req.TableID); err != nil {
			// Non-integer must be usernames?
			if req.TableName == "users" {
				user, err := models.FindUsername(req.TableID)
				if err != nil {
					SendJSON(w, http.StatusBadRequest, Response{
						Error: "User not found.",
					})
					return
				}
				tableID = user.ID
			} else {
				SendJSON(w, http.StatusBadRequest, Response{
					Error: "Invalid ID.",
				})
				return
			}
		} else {
			tableID = uint64(v)
		}

		// Missing TableID?
		if tableID == 0 {
			SendJSON(w, http.StatusBadRequest, Response{
				Error: "Couldn't add a like: invalid table ID.",
			})
			return
		}

		// Who do we notify about this like?
		var (
			targetUser          *models.User
			notificationMessage string
		)
		switch req.TableName {
		case "photos":
			if photo, err := models.GetPhoto(tableID); err == nil {

				// Set the notification link to the /photo/view route.
				linkTo = fmt.Sprintf("/photo/view?id=%d", photo.ID)

				if user, err := models.GetUser(photo.UserID); err == nil {
					// Safety check: if the current user should not see this picture, they can not "Like" it.
					// Example: you unfriended them but they still had the image on their old browser page.
					if ok, _ := photo.ShouldBeSeenBy(currentUser); !ok {
						SendJSON(w, http.StatusForbidden, Response{
							Error: "You are not allowed to like that photo.",
						})
						return
					}

					// Mark this photo as 'viewed' if it received a like.
					// Example: on a gallery view the photo is only 'viewed' if interacted with (lightbox),
					// going straight for the 'Like' button should count as well.
					photo.View(currentUser)

					targetUser = user
				}
			} else {
				log.Error("For like on photos table: didn't find photo %d: %s", tableID, err)
			}
		case "users":
			if user, err := models.GetUser(tableID); err == nil {
				targetUser = user

				// Blocking safety check: if either user blocks the other, liking is not allowed.
				if models.IsBlocking(currentUser.ID, user.ID) {
					SendJSON(w, http.StatusForbidden, Response{
						Error: "You are not allowed to like that profile.",
					})
					return
				}
			} else {
				log.Error("For like on users table: didn't find user %d: %s", tableID, err)
			}
		case "comments":
			if comment, err := models.GetComment(tableID); err == nil {
				targetUser = &comment.User
				notificationMessage = comment.Message

				// Set the notification link to the /go/comment route.
				linkTo = fmt.Sprintf("/go/comment?id=%d", comment.ID)

				// Blocking safety check: if either user blocks the other, liking is not allowed.
				if models.IsBlocking(currentUser.ID, targetUser.ID) {
					SendJSON(w, http.StatusForbidden, Response{
						Error: "You are not allowed to like that comment.",
					})
					return
				}
			} else {
				log.Error("For like on comments table: didn't find comment %d: %s", tableID, err)
			}
		}

		// Is the table likeable?
		if _, ok := models.LikeableTables[req.TableName]; !ok {
			SendJSON(w, http.StatusBadRequest, Response{
				Error: fmt.Sprintf("Can't like table %s: not allowed.", req.TableName),
			})
			return
		}

		// Put in a like.
		if req.Unlike {
			if err := models.Unlike(currentUser, req.TableName, tableID); err != nil {
				SendJSON(w, http.StatusBadRequest, Response{
					Error: fmt.Sprintf("Error unliking: %s", err),
				})
				return
			}

			// Remove the target's notification about this like.
			if targetUser != nil {
				models.RemoveSpecificNotificationAboutUser(targetUser.ID, currentUser.ID, models.NotificationLike, req.TableName, tableID)
			}
		} else {
			if err := models.AddLike(currentUser, req.TableName, tableID); err != nil {
				SendJSON(w, http.StatusBadRequest, Response{
					Error: fmt.Sprintf("Error liking: %s", err),
				})
				return
			}

			// Notify the recipient of the like.
			log.Info("Added like on %s:%d, notifying owner %+v", req.TableName, tableID, targetUser)
			if targetUser != nil && !targetUser.NotificationOptOut(config.NotificationOptOutLikes) {
				notif := &models.Notification{
					UserID:    targetUser.ID,
					AboutUser: *currentUser,
					Type:      models.NotificationLike,
					TableName: req.TableName,
					TableID:   tableID,
					Message:   notificationMessage,
					Link:      linkTo,
				}
				if err := models.CreateNotification(notif); err != nil {
					log.Error("Couldn't create Likes notification: %s", err)
				}
			}
		}

		// Refresh cached like counts.
		if req.TableName == "photos" {
			if err := models.UpdatePhotoCachedCounts(tableID); err != nil {
				log.Error("UpdatePhotoCachedCount(%d): %s", tableID, err)
			}
		}

		// Send success response.
		SendJSON(w, http.StatusOK, Response{
			OK:    true,
			Likes: models.CountLikes(req.TableName, tableID),
		})
	})
}

// WhoLikes API checks who liked something.
func WhoLikes() http.HandlerFunc {
	// Response JSON schema.
	type Liker struct {
		Username     string                  `json:"username"`
		Avatar       string                  `json:"avatar"`
		Relationship models.UserRelationship `json:"relationship"`
	}
	type Response struct {
		OK    bool               `json:"OK"`
		Error string             `json:"error,omitempty"`
		Likes []Liker            `json:"likes,omitempty"`
		Pager *models.Pagination `json:"pager,omitempty"`
		Pages int                `json:"pages,omitempty"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			SendJSON(w, http.StatusNotAcceptable, Response{
				Error: "GET method only",
			})
			return
		}

		// Parse request parameters.
		var (
			tableName   = r.FormValue("table_name")
			tableID, _  = strconv.Atoi(r.FormValue("table_id"))
			friendsOnly = r.FormValue("friends_only")
			page, _     = strconv.Atoi(r.FormValue("page"))
		)
		if tableName == "" {
			SendJSON(w, http.StatusBadRequest, Response{
				Error: "Missing required table_name",
			})
			return
		} else if tableID == 0 {
			SendJSON(w, http.StatusBadRequest, Response{
				Error: "Missing required table_id",
			})
			return
		}

		if page < 1 {
			page = 1
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			SendJSON(w, http.StatusBadRequest, Response{
				Error: "Couldn't get current user!",
			})
			return
		}

		// Get a page of users who've liked this.
		var pager = &models.Pagination{
			Page:    page,
			PerPage: config.PageSizeLikeList,
			Sort:    "created_at desc",
		}
		users, err := models.PaginateLikes(currentUser, tableName, uint64(tableID), friendsOnly, pager)
		if err != nil {
			SendJSON(w, http.StatusInternalServerError, Response{
				Error: fmt.Sprintf("Error getting likes: %s", err),
			})
			return
		}

		// Map user data to just the essentials for front-end.
		var result = []Liker{}
		for _, user := range users {
			result = append(result, Liker{
				Username:     user.Username,
				Avatar:       photo.VisibleAvatarURL(user, currentUser),
				Relationship: user.UserRelationship,
			})
		}

		// Send success response.
		SendJSON(w, http.StatusOK, Response{
			OK:    true,
			Likes: result,
			Pager: pager,
			Pages: pager.Pages(),
		})
	})
}
