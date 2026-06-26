package notification

import (
	"fmt"

	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
)

// Functions to broadcast Notifications (e.g. to your friends about your new photo).

// NotifyFriendsNewPhoto broadcasts a notification about your new photo upload to your friends.
//
// Call this as a goroutine as it can take a while with many friends.
func NotifyFriendsNewPhoto(currentUser *models.User, photo *models.Photo) {
	log.Info("NotifyFriendsNewPhoto for %s", currentUser.Username)

	_, _, notifyUserIDs := GetBroadcastFollowerIDs(
		currentUser,
		"photos",
		photo.Explicit,
		photo.Visibility == models.PhotoFriends,
		photo.Visibility == models.PhotoPrivate,
	)

	for _, fid := range notifyUserIDs {
		notif := &models.Notification{
			UserID:    fid,
			AboutUser: *currentUser,
			Type:      models.NotificationNewPhoto,
			TableName: "photos",
			TableID:   photo.ID,
			Link:      fmt.Sprintf("/photo/view?id=%d", photo.ID),
		}
		if err := models.CreateNotification(notif); err != nil {
			log.Error("Couldn't notify user %d about %s's new photo: %s", fid, currentUser.Username, err)
		}
	}
}

// GetBroadcastFollowerIDs gathers the list of follower IDs that you may broadcast a notification to.
//
// Parameters:
//
// - tableName is like photos, blogs, videos.
// - isExplicit to filter out followers who don't wish to see explicit content.
// - isPrivate to filter for followers who you have unlocked your private content for.
//
// Returns:
//
// 1. Follower IDs (the complete set of followers).
// 2. Notify User IDs (the subset of the followers to notify, taking into account their opt-out preferences).
func GetBroadcastFollowerIDs(currentUser *models.User, tableName string, isExplicit, isFriendsOnly, isPrivate bool) (friendIDs, followerIDs, notifyUserIDs []uint64) {

	// Get the user's literal lists of Friends and Followers (with Explicit opt-ins).
	if isExplicit {
		friendIDs = models.FriendIDsAreExplicit(currentUser.ID)
		followerIDs = models.FollowerIDsAreExplicit(currentUser.ID)
	} else {
		friendIDs = models.FriendIDs(currentUser.ID)
		followerIDs = models.FollowerIDs(currentUser.ID)
	}

	// Who do we notify?
	if isPrivate {
		// Private grantees only.
		if isExplicit {
			notifyUserIDs = models.PrivateGranteeAreExplicitUserIDs(currentUser.ID)
			log.Info("Notify %d EXPLICIT private grantees about new %s by %s", len(notifyUserIDs), tableName, currentUser.Username)
		} else {
			notifyUserIDs = models.PrivateGranteeUserIDs(currentUser.ID)
			log.Info("Notify %d private grantees about new %s by %s", len(notifyUserIDs), tableName, currentUser.Username)
		}
	} else {
		// All of our followers by default.
		notifyUserIDs = followerIDs
	}

	// Filter down the notifyUserIDs to only include the user's followers.
	// Example: someone unlocked private photos for you, but you are not their follower.
	// You should not get notified about their new private photos.
	notifyUserIDs = models.FilterFollowerIDs(notifyUserIDs, followerIDs)

	// Additionally filter down by Friend IDs? For Friends-only content.
	if isFriendsOnly {
		notifyUserIDs = models.FilterFriendIDs(notifyUserIDs, friendIDs)
	}

	// Filter them further to respect specific notification opt-out preferences.
	switch tableName {
	case "photos":
		notifyUserIDs = models.FilterPhotoUploadNotificationUserIDs(
			currentUser,
			isExplicit,
			isPrivate,
			notifyUserIDs,
		)
	default:
		log.Error("GetBroadcastFriendIDs: unexpected tableName: %s", tableName)
	}

	return friendIDs, followerIDs, notifyUserIDs
}
