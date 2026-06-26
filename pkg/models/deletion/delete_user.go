package deletion

import (
	"fmt"

	"github.com/cuvou/gosocial/pkg/chat"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/photo"
)

// DeleteUser wipes a user and all associated data from the database.
func DeleteUser(user *models.User) error {
	log.Error("BEGIN DeleteUser(%d, %s)", user.ID, user.Username)

	// Clear their history on the chat room.
	go func() {
		i, err := chat.EraseChatHistory(user.Username)
		if err != nil {
			log.Error("EraseChatHistory(%s): %s", user.Username, err)
			return
		}

		log.Error("DeleteUser(%s): Cleared chat DMs history for user (%d messages erased)", user.Username, i)
	}()

	// Remove all linked tables and assets.
	type remover struct {
		Step string
		Fn   func(uint64) error
	}

	// Blank out the user's profile photo ID to avoid conflict removing their picture.
	user.RemoveProfilePhoto()

	// Tables to remove. In case of any unexpected DB errors, these tables are ordered
	// to remove the "safest" fields first.
	var todo = []remover{
		{"Admin group memberships", DeleteAdminGroupUsers},
		{"Disown User Forums", DisownForums},
		{"Disown Places", DisownPlaces},
		{"Notifications", DeleteNotifications},
		{"Likes", DeleteLikes},
		{"Threads", DeleteForumThreads},
		{"Comments", DeleteComments},
		{"Subscriptions", DeleteSubscriptions},
		{"Photos", DeleteUserPhotos},
		{"Private Photo Grants", DeletePrivateGrants},
		{"Who's Nearby Locations", DeleteUserLocation},
		{"Comment Photos", DeleteUserCommentPhotos},
		{"Messages", DeleteUserMessages},
		{"Friends", DeleteFriends},
		{"Follows", DeleteFollows},
		{"Blocks", DeleteBlocks},
		{"Feedbacks", DeleteFeedbacks},
		{"Two Factor", DeleteTwoFactor},
		{"Profile Fields", DeleteProfile},
		{"Change Logs", DeleteChangeLogs},
		{"IP Addresses", DeleteIPAddresses},
		{"Push Notifications", DeletePushNotifications},
		{"Forum Memberships", DeleteForumMemberships},
		{"Usage Statistics", DeleteUsageStatistics},
		{"Privacy Settings", DeletePrivacySettings},
		{"Profile Theme", DeleteProfileTheme},
		{"Login Sessions", DeleteLoginSessions},
		{"TaggedUser", DeleteTaggedUsers},
	}
	for _, item := range todo {
		if err := item.Fn(user.ID); err != nil {
			return fmt.Errorf("%s: %s", item.Step, err)
		}
	}

	// Remove the user itself.
	return user.Delete()
}

// DeleteAdminGroupUsers scrubs data for deleting a user.
func DeleteAdminGroupUsers(userID uint64) error {
	log.Error("DeleteUser: DeleteAdminGroupUsers(%d)", userID)
	result := models.DB.Exec(
		"DELETE FROM admin_group_users WHERE user_id = ?",
		userID,
	)
	return result.Error
}

// DeleteUserPhotos scrubs data for deleting a user.
func DeleteUserPhotos(userID uint64) error {
	log.Error("DeleteUser: BEGIN DeleteUserPhotos(%d)", userID)

	// Deeply scrub all user photos.
	pager := &models.Pagination{
		Page:    1,
		PerPage: 20,
		Sort:    "photos.id",
	}

	for {
		photos, err := models.PaginateUserPhotos(
			nil,
			userID,
			models.UserGallery{
				Visibility: models.PhotoVisibilityAll,
			},
			pager,
		)

		if err != nil {
			return err
		}

		if len(photos) == 0 {
			break
		}

		for _, item := range photos {
			log.Warn("DeleteUserPhotos(%d): remove file %s", userID, item.Filename)
			photo.Delete(item.Filename)
			if item.CroppedFilename != "" {
				log.Warn("DeleteUserPhotos(%d): remove file %s", userID, item.CroppedFilename)
				photo.Delete(item.CroppedFilename)
			}
			if err := item.Delete(); err != nil {
				return err
			}
		}
	}

	log.Error("DeleteUser: END DeleteUserPhotos(%d)", userID)
	return nil
}

// DeleteUserCommentPhotos scrubs data for deleting a user.
func DeleteUserCommentPhotos(userID uint64) error {
	log.Error("DeleteUser: BEGIN DeleteUserCommentPhotos(%d)", userID)

	// Deeply scrub all user photos.
	pager := &models.Pagination{
		Page:    1,
		PerPage: 20,
		Sort:    "comment_photos.id",
	}

	for {
		photos, err := models.PaginateUserCommentPhotos(
			userID,
			pager,
		)

		if err != nil {
			return err
		}

		if len(photos) == 0 {
			break
		}

		for _, item := range photos {
			log.Warn("DeleteUserCommentPhotos(%d): remove file %s", userID, item.Filename)
			photo.Delete(item.Filename)
			if err := item.Delete(); err != nil {
				return err
			}
		}
	}

	log.Error("DeleteUser: END DeleteUserPhotos(%d)", userID)
	return nil
}

// DeleteTwoFactor scrubs data for deleting a user.
func DeleteTwoFactor(userID uint64) error {
	log.Error("DeleteUser: DeleteTwoFactor(%d)", userID)
	result := models.DB.Where(
		"user_id = ?",
		userID,
	).Delete(&models.TwoFactor{})
	return result.Error
}

// DeleteUserLocation scrubs data for deleting a user.
func DeleteUserLocation(userID uint64) error {
	log.Error("DeleteUser: DeleteUserLocation(%d)", userID)
	result := models.DB.Where(
		"user_id = ?",
		userID,
	).Delete(&models.UserLocation{})
	return result.Error
}

// DeleteUserMessages scrubs data for deleting a user.
func DeleteUserMessages(userID uint64) error {
	log.Error("DeleteUser: DeleteUserMessages(%d)", userID)
	result := models.DB.Where(
		"source_user_id = ? OR target_user_id = ?",
		userID, userID,
	).Delete(&models.Message{})
	return result.Error
}

// DeleteFriends scrubs data for deleting a user.
func DeleteFriends(userID uint64) error {
	log.Error("DeleteUser: DeleteUserFriends(%d)", userID)
	result := models.DB.Where(
		"source_user_id = ? OR target_user_id = ?",
		userID, userID,
	).Delete(&models.Friend{})
	return result.Error
}

// DeleteFollows scrubs data for deleting a user.
func DeleteFollows(userID uint64) error {
	log.Error("DeleteUser: DeleteFollows(%d)", userID)
	result := models.DB.Where(
		"source_user_id = ? OR target_user_id = ?",
		userID, userID,
	).Delete(&models.Follow{})
	return result.Error
}

// DeleteBlocks scrubs data for deleting a user.
func DeleteBlocks(userID uint64) error {
	log.Error("DeleteUser: DeleteBlocks(%d)", userID)
	result := models.DB.Where(
		"source_user_id = ? OR target_user_id = ?",
		userID, userID,
	).Delete(&models.Block{})
	return result.Error
}

// DeleteFeedbacks scrubs data for deleting a user.
func DeleteFeedbacks(userID uint64) error {
	log.Error("DeleteUser: DeleteFeedbacks(%d)", userID)
	result := models.DB.Where(
		"user_id = ? OR (table_name='users' AND table_id=?)",
		userID, userID,
	).Delete(&models.Feedback{})
	return result.Error
}

// DeletePrivateGrants scrubs data for deleting a user.
func DeletePrivateGrants(userID uint64) error {
	log.Error("DeleteUser: DeletePrivateGrants(%d)", userID)
	result := models.DB.Where(
		"source_user_id = ? OR target_user_id = ?",
		userID, userID,
	).Delete(&models.PrivatePhoto{})
	return result.Error
}

// DeleteNotifications scrubs all notifications about a user.
func DeleteNotifications(userID uint64) error {
	log.Error("DeleteUser: DeleteNotifications(%d)", userID)
	result := models.DB.Where(
		"user_id = ? OR about_user_id = ?",
		userID, userID,
	).Delete(&models.Notification{})
	return result.Error
}

// DeleteSubscriptions scrubs all notification subscriptions about a user.
func DeleteSubscriptions(userID uint64) error {
	log.Error("DeleteUser: DeleteSubscriptions(%d)", userID)
	result := models.DB.Where(
		"user_id = ?",
		userID,
	).Delete(&models.Subscription{})
	return result.Error
}

// DeleteLikes scrubs all Likes about a user.
func DeleteLikes(userID uint64) error {
	log.Error("DeleteUser: DeleteLikes(%d)", userID)
	result := models.DB.Where(
		"user_id = ? OR (table_name='users' AND table_id=?)",
		userID, userID,
	).Delete(&models.Like{})
	return result.Error
}

// DeleteProfile scrubs data for deleting a user.
func DeleteProfile(userID uint64) error {
	log.Error("DeleteUser: DeleteProfile(%d)", userID)
	result := models.DB.Where(
		"user_id = ?",
		userID,
	).Delete(&models.ProfileField{})
	return result.Error
}

// DeleteForumThreads scrubs all forum threads started by the user.
func DeleteForumThreads(userID uint64) error {
	log.Error("DeleteUser: DeleteForumThreads(%d)", userID)

	var threadIDs = []uint64{}
	result := models.DB.Table(
		"threads",
	).Joins(
		"JOIN comments ON (threads.comment_id = comments.id)",
	).Select(
		"distinct(threads.id) as id",
	).Where(
		"comments.user_id = ?",
		userID,
	).Scan(&threadIDs)

	if result.Error != nil {
		return fmt.Errorf("Couldn't list thread IDs created by user: %s", result.Error)
	}

	log.Warn("thread IDs to wipe: %+v", threadIDs)

	// Wipe all these threads and their comments.
	if len(threadIDs) > 0 {
		// First, delete the threads - so they won't be referring to comment_ids that we
		// delete next and causing errors to arise.
		result = models.DB.Where(
			"id IN ?",
			threadIDs,
		).Delete(&models.Thread{})
		if result.Error != nil {
			return fmt.Errorf("Couldn't delete your forum threads: %s", result.Error)
		}

		// Remove the comments.
		result = models.DB.Where(
			"table_name = ? AND table_id IN ?",
			"threads", threadIDs,
		).Delete(&models.Comment{})
		if result.Error != nil {
			return fmt.Errorf("Couldn't wipe threads of comments: %s", result.Error)
		}

		return result.Error
	}

	return nil
}

// DeleteComments deletes all comments by the user.
func DeleteComments(userID uint64) error {
	log.Error("DeleteUser: DeleteComments(%d)", userID)

	result := models.DB.Where(
		"user_id = ?",
		userID,
	).Delete(&models.Comment{})
	return result.Error
}

// DeleteChangeLogs scrubs data for deleting a user.
func DeleteChangeLogs(userID uint64) error {
	log.Error("DeleteUser: DeleteChangeLogs(%d)", userID)
	result := models.DB.Where(
		"about_user_id = ?",
		userID,
	).Delete(&models.ChangeLog{})
	return result.Error
}

// DeleteIPAddresses scrubs data for deleting a user.
func DeleteIPAddresses(userID uint64) error {
	log.Error("DeleteUser: DeleteIPAddresses(%d)", userID)
	result := models.DB.Where(
		"user_id = ?",
		userID,
	).Delete(&models.IPAddress{})
	return result.Error
}

// DeletePushNotifications scrubs data for deleting a user.
func DeletePushNotifications(userID uint64) error {
	log.Error("DeleteUser: DeletePushNotifications(%d)", userID)
	result := models.DB.Where(
		"user_id = ?",
		userID,
	).Delete(&models.PushNotification{})
	return result.Error
}

// DisownForums unlinks the user from their owned forums.
func DisownForums(userID uint64) error {
	log.Error("DeleteUser: DisownForums(%d)", userID)
	result := models.DB.Exec(`
		UPDATE forums
		SET owner_id = NULL
		WHERE owner_id = ?
	`, userID)
	return result.Error
}

// DisownPlaces unlinks the user from their created Places.
func DisownPlaces(userID uint64) error {
	log.Error("DeleteUser: DisownPlaces(%d)", userID)
	result := models.DB.Exec(`
		UPDATE places
		SET user_id = NULL
		WHERE user_id = ?
	`, userID)
	return result.Error
}

// DeleteForumMemberships scrubs data for deleting a user.
func DeleteForumMemberships(userID uint64) error {
	log.Error("DeleteUser: DeleteForumMemberships(%d)", userID)
	result := models.DB.Where(
		"user_id = ?",
		userID,
	).Delete(&models.ForumMembership{})
	return result.Error
}

// DeleteUsageStatistics scrubs data for deleting a user.
func DeleteUsageStatistics(userID uint64) error {
	log.Error("DeleteUser: DeleteUsageStatistics(%d)", userID)
	result := models.DB.Where(
		"user_id = ?",
		userID,
	).Delete(&models.UsageStatistic{})
	return result.Error
}

// DeletePrivacySettings scrubs data for deleting a user.
func DeletePrivacySettings(userID uint64) error {
	log.Error("DeleteUser: DeletePrivacySettings(%d)", userID)
	result := models.DB.Where(
		"user_id = ?",
		userID,
	).Delete(&models.PrivacySetting{})
	return result.Error
}

// DeleteProfileTheme scrubs data for deleting a user.
func DeleteProfileTheme(userID uint64) error {
	log.Error("DeleteUser: DeleteProfileTheme(%d)", userID)
	result := models.DB.Where(
		"user_id = ?",
		userID,
	).Delete(&models.ProfileTheme{})
	return result.Error
}

// DeleteLoginSessions scrubs data for deleting a user.
func DeleteLoginSessions(userID uint64) error {
	log.Error("DeleteUser: DeleteLoginSessions(%d)", userID)
	result := models.DB.Where(
		"user_id = ?",
		userID,
	).Delete(&models.LoginSession{})
	return result.Error
}

// DeleteTaggedUsers scrubs data for deleting a user.
func DeleteTaggedUsers(userID uint64) error {
	log.Error("DeleteUser: DeleteTaggedUsers(%d)", userID)
	result := models.DB.Where(
		"user_id = ?",
		userID,
	).Delete(&models.TaggedUser{})
	return result.Error
}
