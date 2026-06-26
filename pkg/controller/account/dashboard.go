package account

import (
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/notification"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// GetGroupedNotificationSetting parses the group query parameter, stores the preference
// for the user in profile fields, and returns the current boolean setting (default true).
func GetGroupedNotificationSetting(r *http.Request, currentUser *models.User) bool {
	// Is the user grouping their notifications?
	group := r.FormValue("group")
	switch group {
	case "":
		// Restore it from their permanent saved setting until they toggle it back.
		group = currentUser.GetProfileField("notification_grouping_default")
	case "true", "false":
		// Store their express setting.
		currentUser.SetProfileField("notification_grouping_default", group)
	}

	// The default = true.
	if group == "" {
		group = "true"
	}

	// The boolean setting: are we grouping/compressing the notifications?
	return group == "true"
}

// User dashboard or landing page (/me).
func Dashboard() http.HandlerFunc {
	tmpl := templates.Must("account/dashboard.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			http.Error(w, "Couldn't get currentUser", http.StatusInternalServerError)
			return
		}

		// Parse the 'before_id' parameter.
		var beforeID uint64
		if i, err := strconv.Atoi(r.FormValue("before_id")); err == nil {
			beforeID = uint64(i)
		}

		// Mark all notifications read?
		if r.Method == http.MethodPost {
			switch r.FormValue("intent") {
			case "read-notifications":
				if err := models.MarkNotificationsRead(currentUser); err != nil {
					session.FlashError(w, r, "Error marking your notifications as read: %s", err)
				} else {
					session.Flash(w, r, "All of your notifications have been marked as 'read!'")
				}
			case "clear-all":
				if err := models.ClearAllNotifications(currentUser); err != nil {
					session.FlashError(w, r, "Error clearing your notifications: %s", err)
				} else {
					session.Flash(w, r, "All of your notifications have been cleared!")
				}
			default:
				session.FlashError(w, r, "Unknown intent.")
			}

			templates.Redirect(w, r.URL.Path)
			return
		}

		// Parse notification filters.
		nf := models.NewNotificationFilterFromForm(r)

		// Will the notifications be grouped? (Similar "like" notifications compressed into one)
		areNotificationsGrouped := GetGroupedNotificationSetting(r, currentUser)

		// Page size (depending on whether we ungroup or compress notifications).
		var pageSize = config.PageSizeNotificationsQuery
		if !areNotificationsGrouped {
			pageSize = config.PageSizeNotificationsShow
		}

		// Get our notifications.
		notifs, err := models.PaginateNotifications(currentUser, nf, pageSize, beforeID)
		if err != nil {
			session.FlashError(w, r, "Couldn't get your notifications: %s", err)
		}

		// Populate user relationships in DB models.
		models.SetUserRelationshipsInNotifications(currentUser, notifs)

		// Map likes for in-line like buttons on (other peoples) photos.
		// NOTE: comments can be trickier since the Notification.table_name='photos' if the comment is on a photo,
		// hard to create a LikesMap for the specific comment ID.
		var (
			photoIDs = []uint64{}
			users    = []*models.User{}
		)
		for _, notif := range notifs {
			switch notif.TableName {
			case "photos":
				photoIDs = append(photoIDs, notif.TableID)
			}

			users = append(users, &notif.AboutUser)
		}

		// Get the front-end view of the user's notifications.
		notifications := notification.FromModels(currentUser, r, notifs)
		if areNotificationsGrouped {
			notifications = notification.Compress(notifications, config.PageSizeNotificationsShow)
		}

		// For the 'next page' we switch to cursor-based because of the compression.
		var nextPageID uint64
		if len(notifications) > 0 {
			IDs := notifications[len(notifications)-1].IDs
			if len(IDs) > 0 {
				nextPageID = IDs[len(IDs)-1]
			}
		}

		var vars = map[string]any{
			// Notifications mapped to the front-end structs.
			"Notifications": notifications,
			"NextPageID":    nextPageID,

			"Filters":                 nf,
			"AreNotificationsGrouped": areNotificationsGrouped,

			"PhotoLikeMap": models.MapLikes(currentUser, "photos", photoIDs),
			"FriendMap":    models.MapFriends(currentUser, users),

			// Check 2FA enabled status for new feature announcement.
			"TwoFactorEnabled": models.Get2FA(currentUser.ID).Enabled,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
