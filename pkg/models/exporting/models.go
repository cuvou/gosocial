package exporting

import (
	"archive/zip"
	"fmt"

	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"gorm.io/gorm"
)

// ExportModels is the entry point function to export all data tables about a user.
func ExportModels(zw *zip.Writer, user *models.User) error {
	type task struct {
		Step string
		Fn   func(*zip.Writer, *models.User) error
	}

	// List of tables to export. Keep the ordering in sync with
	// the AutoMigrate() calls in ../models.go
	var todo = []task{
		// Note: AdminGroup info is eager-loaded in User export
		{"Block", ExportBlockTable},
		{"ChangeLog", ExportChangeLogTable},
		{"Comment", ExportCommentTable},
		{"CommentPhoto", ExportCommentPhotoTable},
		{"Feedback", ExportFeedbackTable},
		{"ForumMembership", ExportForumMembershipTable},
		{"Friend", ExportFriendTable},
		{"Follow", ExportFollowTable},
		{"Forum", ExportForumTable},
		{"IPAddress", ExportIPAddressTable},
		{"Like", ExportLikeTable},
		{"Message", ExportMessageTable},
		{"Notification", ExportNotificationTable},
		{"ProfileField", ExportProfileFieldTable},
		{"Photo", ExportPhotoTable},
		// Note: Poll table is eager-loaded in Thread export
		{"PollVote", ExportPollVoteTable},
		{"PrivatePhoto", ExportPrivatePhotoTable},
		{"PushNotification", ExportPushNotificationTable},
		{"Subscription", ExportSubscriptionTable},
		{"Thread", ExportThreadTable},
		{"TwoFactor", ExportTwoFactorTable},
		{"UsageStatistic", ExportUsageStatisticTable},
		{"User", ExportUserTable},
		{"UserLocation", ExportUserLocationTable},
		{"TaggedUser", ExportTaggedUserTable},

		// BareRTC DMs.
		{"DirectMessage", ExportDirectMessageTable},
	}
	for _, item := range todo {
		log.Info("Exporting data model: %s", item.Step)
		if err := item.Fn(zw, user); err != nil {
			return fmt.Errorf("%s: %s", item.Step, err)
		}
	}

	return nil
}

func ExportUserTable(zw *zip.Writer, user *models.User) error {
	return ZipJson(zw, "user.json", user)
}

func ExportProfileFieldTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.ProfileField{}
		query = models.DB.Model(&models.ProfileField{}).Where(
			"user_id = ?",
			user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "profile_fields.json", items)
}

func ExportPhotoTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.Photo{}
		query = models.DB.Model(&models.Photo{}).Where(
			"user_id = ?",
			user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	// Copy all the images into the ZIP.
	for _, row := range items {
		if row.Filename != "" {
			if err := ZipPhoto(zw, "photos", row.Filename); err != nil {
				return err
			}
		}
		if row.CroppedFilename != "" {
			if err := ZipPhoto(zw, "profile_photos", row.CroppedFilename); err != nil {
				return err
			}
		}
	}

	return ZipJson(zw, "photos.json", items)
}

func ExportPrivatePhotoTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.PrivatePhoto{}
		query = models.DB.Model(&models.PrivatePhoto{}).Where(
			"source_user_id = ? OR target_user_id = ?",
			user.ID, user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "private_photos.json", items)
}

func ExportMessageTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.Message{}
		query = models.DB.Model(&models.Message{}).Where(
			"source_user_id = ? OR target_user_id = ?",
			user.ID, user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "messages.json", items)
}

func ExportFriendTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.Friend{}
		query = models.DB.Model(&models.Friend{}).Where(
			"source_user_id = ? OR target_user_id = ?",
			user.ID, user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "friends.json", items)
}

func ExportFollowTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.Follow{}
		query = models.DB.Model(&models.Follow{}).Where(
			"source_user_id = ? OR target_user_id = ?",
			user.ID, user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "follows.json", items)
}

func ExportBlockTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.Block{}
		query = models.DB.Model(&models.Block{}).Where(
			"source_user_id = ? OR target_user_id = ?",
			user.ID, user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "blocks.json", items)
}

func ExportFeedbackTable(zw *zip.Writer, user *models.User) error {
	var (
		items       = []*models.Feedback{}
		photoIDs, _ = user.AllPhotoIDs()
		query       *gorm.DB
	)

	// If they have photos, query on those.
	if len(photoIDs) > 0 {
		query = models.DB.Model(&models.Feedback{}).Where(
			"user_id = ? OR (table_name = 'users' AND table_id = ?) OR (table_name = 'photos' AND table_id IN ?)",
			user.ID, user.ID, photoIDs,
		).Find(&items)
	} else {
		// Only reports about their user.
		query = models.DB.Model(&models.Feedback{}).Where(
			"user_id = ? OR (table_name = 'users' AND table_id = ?)",
			user.ID, user.ID,
		).Find(&items)
	}

	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "feedback.json", items)
}

func ExportForumTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.Forum{}
		query = models.DB.Model(&models.Forum{}).Where(
			"owner_id = ?",
			user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "forums.json", items)
}

func ExportThreadTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.Thread{}
		query = (&models.Thread{}).Preload().Joins(
			"JOIN comments ON (comments.id = threads.comment_id)",
		).Where(
			"comments.user_id = ?",
			user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "threads.json", items)
}

func ExportCommentTable(zw *zip.Writer, user *models.User) error {
	var (
		items       = []*models.Comment{}
		photoIDs, _ = user.AllPhotoIDs()
		query       *gorm.DB
	)

	// If they have photos, query on those.
	if len(photoIDs) > 0 {
		query = models.DB.Model(&models.Comment{}).Where(
			"user_id = ? OR (table_name = 'users' AND table_id = ?) OR (table_name = 'photos' AND table_id IN ?)",
			user.ID, user.ID, photoIDs,
		).Find(&items)
	} else {
		query = models.DB.Model(&models.Comment{}).Where(
			"user_id = ? OR (table_name = 'users' AND table_id = ?)",
			user.ID, user.ID,
		).Find(&items)
	}

	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "comments.json", items)
}

func ExportLikeTable(zw *zip.Writer, user *models.User) error {
	var (
		items       = []*models.Like{}
		photoIDs, _ = user.AllPhotoIDs()
		query       *gorm.DB
	)

	// If they have photos, query on those.
	if len(photoIDs) > 0 {
		query = models.DB.Model(&models.Like{}).Where(
			"user_id = ? OR (table_name = 'users' AND table_id = ?) OR (table_name = 'photos' AND table_id IN ?)",
			user.ID, user.ID, photoIDs,
		).Find(&items)
	} else {
		// Only reports about their user.
		query = models.DB.Model(&models.Like{}).Where(
			"user_id = ? OR (table_name = 'users' AND table_id = ?)",
			user.ID, user.ID,
		).Find(&items)
	}

	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "likes.json", items)
}

func ExportNotificationTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.Notification{}
		query = models.DB.Model(&models.Notification{}).Where(
			"user_id = ?",
			user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "notifications.json", items)
}

func ExportSubscriptionTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.Subscription{}
		query = models.DB.Model(&models.Subscription{}).Where(
			"user_id = ?",
			user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "subscriptions.json", items)
}

func ExportCommentPhotoTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.CommentPhoto{}
		query = models.DB.Model(&models.CommentPhoto{}).Where(
			"user_id = ?",
			user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	// Copy all the images into the ZIP.
	for _, row := range items {
		if row.Filename != "" {
			if err := ZipPhoto(zw, "comment_photos", row.Filename); err != nil {
				return err
			}
		}
	}

	return ZipJson(zw, "comment_photos.json", items)
}

func ExportPollVoteTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.PollVote{}
		query = (&models.PollVote{}).Preload().Where(
			"user_id = ?",
			user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "poll_votes.json", items)
}

func ExportChangeLogTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.ChangeLog{}
		query = models.DB.Model(&models.ChangeLog{}).Where(
			"about_user_id = ? OR admin_user_id = ?",
			user.ID, user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "change_logs.json", items)
}

func ExportUserLocationTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.UserLocation{}
		query = models.DB.Model(&models.UserLocation{}).Where(
			"user_id = ?",
			user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "user_location.json", items)
}

func ExportTwoFactorTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.TwoFactor{}
		query = models.DB.Model(&models.TwoFactor{}).Where(
			"user_id = ?",
			user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "two_factor.json", items)
}

func ExportIPAddressTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.IPAddress{}
		query = models.DB.Model(&models.IPAddress{}).Where(
			"user_id = ?",
			user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "ip_addresses.json", items)
}

func ExportForumMembershipTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.ForumMembership{}
		query = models.DB.Model(&models.ForumMembership{}).Where(
			"user_id = ?",
			user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "forum_memberships.json", items)
}

func ExportPushNotificationTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.PushNotification{}
		query = models.DB.Model(&models.PushNotification{}).Where(
			"user_id = ?",
			user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "push_notifications.json", items)
}

func ExportUsageStatisticTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.UsageStatistic{}
		query = models.DB.Model(&models.UsageStatistic{}).Where(
			"user_id = ?",
			user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "usage_statistics.json", items)
}

func ExportDirectMessageTable(zw *zip.Writer, user *models.User) error {
	var (
		likes = []string{
			fmt.Sprintf(`@%s:%%`, user.Username),
			fmt.Sprintf(`%%:@%s`, user.Username),
		}
		items = []*models.DirectMessage{}
		query = models.DB.Model(&models.DirectMessage{}).Where(
			"channel_id LIKE ? OR channel_id LIKE ?",
			likes[0], likes[1],
		).Order("channel_id, created_at").Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "direct_messages.json", items)
}

func ExportTaggedUserTable(zw *zip.Writer, user *models.User) error {
	var (
		items = []*models.TaggedUser{}
		query = models.DB.Model(&models.TaggedUser{}).Where(
			"user_id = ?",
			user.ID,
		).Find(&items)
	)
	if query.Error != nil {
		return query.Error
	}

	return ZipJson(zw, "tagged_users.json", items)
}
