package photo

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Private controller (/photo/private) to see and modify your Private Photo grants.
func Private() http.HandlerFunc {
	// Reuse the upload page but with an EditPhoto variable.
	tmpl := templates.Must("photo/private.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			view      = r.FormValue("view")
			isGrantee = view == "grantee"
		)

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: could not get currentUser.")
			templates.Redirect(w, "/")
			return
		}

		// Get the users.
		pager := &models.Pagination{
			PerPage: config.PageSizePrivatePhotoGrantees,
			Sort:    "updated_at desc",
		}
		pager.ParsePage(r)
		users, err := models.PaginatePrivatePhotoList(currentUser, isGrantee, pager)
		if err != nil {
			session.FlashError(w, r, "Couldn't paginate users: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Collect user IDs for some mappings.
		var userIDs = []uint64{}
		for _, user := range users {
			userIDs = append(userIDs, user.ID)
		}

		// Map reverse grantee statuses.
		var GranteeMap interface{}
		if isGrantee {
			// Shared With Me page: map whether we grant them shares back.
			GranteeMap = models.MapPrivatePhotoGranted(currentUser, users)
		} else {
			// My Shares page: map whether they share back with us.
			GranteeMap = models.MapPrivatePhotoGrantee(currentUser, users)
		}

		canSharePrivatePhotos, _ := models.ShouldShowPrivateUnlockPrompt(currentUser, nil)

		var vars = map[string]interface{}{
			"IsGrantee":    isGrantee,
			"CountGrantee": models.CountPrivateGrantee(currentUser.ID),
			"GranteeMap":   GranteeMap,
			"Users":        users,
			"Pager":        pager,

			// Does the current user have any private photos to share?
			"CanSharePrivatePhotos": canSharePrivatePhotos,

			// Mapped user statuses for frontend cards.
			"PhotoCountMap": models.MapPhotoCountsByVisibility(users, models.PhotoPrivate),
			"FriendMap":     models.MapFriends(currentUser, users),
			"LikedMap":      models.MapLikes(currentUser, "users", userIDs),
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

// Share your private photos with a new user.
func Share() http.HandlerFunc {
	tmpl := templates.Must("photo/share.html")

	// Whitelist for ordering your friend list here.
	var sortWhitelist = []string{
		"updated_at desc",
		"updated_at asc",
		"users.username asc",
		"users.username desc",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// To whom?
		var (
			user        *models.User
			username    = strings.TrimSpace(strings.ToLower(r.FormValue("to")))
			isRevokeAll = r.FormValue("intent") == "revoke-all"

			// Sorting the friend list.
			sort   = r.FormValue("sort")
			sortOK bool
		)

		// Sort options.
		for _, v := range sortWhitelist {
			if sort == v {
				sortOK = true
				break
			}
		}
		if !sortOK {
			sort = sortWhitelist[0]
		}

		if username != "" {
			if u, err := models.FindUsername(username); err != nil {
				session.FlashError(w, r, "That username was not found, please try again.")
				templates.Redirect(w, r.URL.Path)
				return
			} else {
				user = u
			}
		}

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: could not get currentUser.")
			templates.Redirect(w, "/")
			return
		}

		// Are we revoking our privates from ALL USERS?
		if isRevokeAll {
			// Revoke any "has uploaded a new private photo" notifications from all users' lists.
			if err := models.RevokePrivatePhotoNotifications(currentUser, nil); err != nil {
				log.Error("RevokePrivatePhotoNotifications(%s): %s", currentUser.Username, err)
			}

			models.RevokePrivatePhotosAll(currentUser.ID)
			session.Flash(w, r, "Your private photos have been locked from ALL users.")
			templates.Redirect(w, "/photo/private")

			// Remove ALL notifications sent to ALL users who had access before.
			models.RemoveNotification("__private_photos", currentUser.ID)

			// Log the change.
			models.LogDeleted(currentUser, nil, "private_photos", 0, "Revoked ALL private photo shares.", nil)
			return
		}

		if user != nil && currentUser.ID == user.ID {
			session.FlashError(w, r, "You cannot share your private photos with yourself.")
			templates.Redirect(w, r.URL.Path)
			return
		}

		// Any blocking?
		if user != nil && models.IsBlocking(currentUser.ID, user.ID) && !currentUser.IsAdmin {
			session.FlashError(w, r, "You are blocked from contacting this user.")
			templates.Redirect(w, r.URL.Path)
			return
		}

		// Do we have any privates to share?
		canSharePrivatePhotos, _ := models.ShouldShowPrivateUnlockPrompt(currentUser, user)

		// POSTing?
		if r.Method == http.MethodPost {
			var (
				intent = r.PostFormValue("intent")
			)

			if user == nil {
				session.FlashError(w, r, "Did not find a username to share with!")
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Is the recipient blocking this photo share?
			if intent != "decline" && intent != "revoke" {
				if !canSharePrivatePhotos {
					session.FlashError(w, r, "You are unable to share your private photos with %s.", user.Username)
					templates.Redirect(w, "/u/"+user.Username)
					return
				}
			}

			// If submitting, do it and redirect.
			if intent == "submit" {
				models.UnlockPrivatePhotos(currentUser.ID, user.ID)
				session.Flash(w, r, "Your private photos have been unlocked for %s.", user.Username)
				templates.Redirect(w, "/photo/private")

				// Create a notification for this.
				if !user.NotificationOptOut(config.NotificationOptOutPrivateGrant) {
					notif := &models.Notification{
						UserID:    user.ID,
						AboutUser: *currentUser,
						Type:      models.NotificationPrivatePhoto,
						TableName: "__private_photos",
						TableID:   currentUser.ID,
						Link:      fmt.Sprintf("/u/%s/photos?visibility=private", currentUser.Username),
					}
					if err := models.CreateNotification(notif); err != nil {
						log.Error("Couldn't create PrivatePhoto notification: %s", err)
					}
				}

				// Log the change.
				models.LogCreated(currentUser, "private_photos", user.ID, fmt.Sprintf("Private Photo access granted to @%s", user.Username))

				return
			} else if intent == "revoke" {
				models.RevokePrivatePhotos(currentUser.ID, user.ID)
				session.Flash(w, r, "You have revoked access to your private photos for %s.", user.Username)
				templates.Redirect(w, "/photo/private")

				// Remove any notification we created when the grant was given.
				models.RemoveSpecificNotification(user.ID, models.NotificationPrivatePhoto, "__private_photos", currentUser.ID)

				// Revoke any "has uploaded a new private photo" notifications in this user's list.
				if err := models.RevokePrivatePhotoNotifications(currentUser, user); err != nil {
					log.Error("RevokePrivatePhotoNotifications(%s): %s", currentUser.Username, err)
				}

				// Log the change.
				models.LogDeleted(currentUser, nil, "private_photos", user.ID, fmt.Sprintf("Revoked Private Photo access to user @%s", user.Username), nil)

				return
			} else if intent == "decline" {
				// Decline = they shared with me and we do not want it.
				models.RevokePrivatePhotos(user.ID, currentUser.ID)
				session.Flash(w, r, "You have declined access to see %s's private photos.", user.Username)

				// Remove any notification we created when the grant was given.
				models.RemoveSpecificNotification(currentUser.ID, models.NotificationPrivatePhoto, "__private_photos", user.ID)

				// Revoke any "has uploaded a new private photo" notifications in this user's list.
				if err := models.RevokePrivatePhotoNotifications(user, currentUser); err != nil {
					log.Error("RevokePrivatePhotoNotifications(%s): %s", user.Username, err)
				}

				templates.Redirect(w, "/photo/private?view=grantee")

				// Log the change.
				models.LogDeleted(currentUser, nil, "private_photos", user.ID, fmt.Sprintf("Declined a Private Photo share by @%s", user.Username), nil)
				return
			}

			// The other intent is "preview" so the user gets the confirmation
			// screen before they continue, which shows the selected user info.
		}

		// Paginate the user's Friends list for easy sharing.
		var pager = &models.Pagination{
			PerPage: config.PageSizePrivateShareFriends,
			Sort:    sort,
		}
		pager.ParsePage(r)

		friends, err := models.PaginateFriends(currentUser, false, false, false, pager)
		if err != nil {
			session.FlashError(w, r, "Getting your friends list: %s", err)
		}

		// Of these friends: map which ones we have already granted.
		granteeMap := models.MapPrivatePhotoGranted(currentUser, friends)

		var vars = map[string]any{
			"User": user,

			// Count of current user's private photos, e.g. to alert them that they have
			// none and so can not unlock their privates.
			"CanSharePrivatePhotos": canSharePrivatePhotos,

			// Friends List
			"Friends":    friends,
			"GranteeMap": granteeMap,
			"Sort":       sort,
			"Pager":      pager,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
