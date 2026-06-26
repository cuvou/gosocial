package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"gorm.io/gorm"
)

// Notification table.
type Notification struct {
	ID          uint64           `gorm:"primaryKey"`
	UserID      uint64           `gorm:"index"` // who it belongs to
	AboutUserID *uint64          `form:"index"` // the other party of this notification
	AboutUser   User             `gorm:"foreignKey:about_user_id"`
	Type        NotificationType `gorm:"index"` // like, comment, ...
	Read        bool             `gorm:"index"`
	TableName   string           `gorm:"index:idx_notification_table"` // on which of your tables (photos, comments, ...)
	TableID     uint64           `gorm:"index:idx_notification_table"`
	Message     string           // text associated, e.g. copy of comment added
	Link        string           // associated URL, e.g. for comments
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Preload related tables for the forum (classmethod).
func (n *Notification) Preload() *gorm.DB {
	return DB.Preload("AboutUser.ProfilePhoto")
}

type NotificationType string

// Notification Types.
// SEE ALSO: the mapping in notification_filters.go.
const (
	NotificationLike           NotificationType = "like"
	NotificationFriendApproved NotificationType = "friendship_approved"
	NotificationComment        NotificationType = "comment"
	NotificationAlsoCommented  NotificationType = "also_comment"
	NotificationAlsoPosted     NotificationType = "also_posted"   // forum replies
	NotificationPrivatePhoto   NotificationType = "private_photo" // private photo grants
	NotificationNewPhoto       NotificationType = "new_photo"
	NotificationForumModerator NotificationType = "forum_moderator" // appointed as a forum moderator
	NotificationFollow         NotificationType = "follow"
	NotificationFollowBack     NotificationType = "follow_back"
	NotificationTaggedUser     NotificationType = "tagged_user"
	NotificationCustom         NotificationType = "custom" // custom message pushed
)

// CreateNotification inserts a new notification into the database.
func CreateNotification(n *Notification) error {
	// Insert via raw SQL query, reasoning:
	// the AboutUser relationship has gorm do way too much work:
	// - Upsert the user profile photo
	// - Upsert the user profile fields
	// - Upsert the user row itself
	// .. and if we notify all your friends, all these wasteful queries ran
	// for every single notification created!
	if n.AboutUserID == nil && n.AboutUser.ID > 0 {
		n.AboutUserID = &n.AboutUser.ID
	}
	return DB.Exec(
		`
		INSERT INTO notifications
		(user_id, about_user_id, type, read, table_name, table_id, message, link, created_at, updated_at)
		VALUES
		(?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		`,
		n.UserID,
		n.AboutUserID,
		n.Type,
		false,
		n.TableName,
		n.TableID,
		n.Message,
		n.Link,
		time.Now(),
		time.Now(),
	).Error
}

// GetNotification by ID.
func GetNotification(id uint64) (*Notification, error) {
	var n *Notification
	result := DB.Model(n).First(&n, id)
	return n, result.Error
}

// NotificationOptOut checks whether the user opts-out of a class of notification.
func (u *User) NotificationOptOut(name string) bool {
	return u.GetProfileField(name) == "true"
}

// RemoveNotification about a table ID, e.g. when removing a like.
func RemoveNotification(tableName string, tableID uint64) error {
	result := DB.Where(
		"table_name = ? AND table_id = ?",
		tableName, tableID,
	).Delete(&Notification{})
	return result.Error
}

// RemoveAlsoPostedNotification removes a 'has also posted' notification if the comment is later deleted.
//
// This is specialized for deleting replies to forum threads where subscribers were notified that the
// user has AlsoPosted on that thread. If the user deletes their comment, this specific notification
// needs to be revoked from people who received it before, so the head of their original comment is not
// leaked on their notifications page.
//
// These notifications have a Type=also_posted TableName=threads TableID=threads.ID with the only hard
// link to the specific comment on that thread being the hyperlink URL that goes to their comment.
func RemoveAlsoPostedNotification(thread *Thread, commentID uint64) error {
	// Match the specific notification by its link URL.
	var (
		// Modern link URL ('/go/comment?id=1234' which finds the right page to see the comment)
		newLink = fmt.Sprintf("/go/comment?id=%d", commentID)

		// Legacy link URL ('/forum/thread/123?page=4#p456') which embeds the thread ID, an
		// optional query string (page number) and the comment ID anchor.
		legacyLink = fmt.Sprintf("/forum/thread/%d%%#p%d", thread.ID, commentID)
	)

	result := DB.Where(
		`
			(
				type = ? AND table_name = 'threads' AND table_id = ? AND (link = ? OR link LIKE ?)
			) OR (
				table_name = 'comments' AND table_id = ?
			)
		`,
		NotificationAlsoPosted, thread.ID, newLink, legacyLink, commentID,
	).Delete(&Notification{})
	return result.Error
}

// RemoveNotificationBulk about several table IDs, e.g. when bulk removing private photo upload
// notifications for everybody on the site.
func RemoveNotificationBulk(tableName string, tableIDs []uint64) error {
	result := DB.Where(
		"table_name = ? AND table_id IN ?",
		tableName, tableIDs,
	).Delete(&Notification{})
	return result.Error
}

// RemoveSpecificNotification to remove more specialized notifications where just removing by
// table name+ID is not adequate, e.g. for Private Photo Unlocks.
func RemoveSpecificNotification(userID uint64, t NotificationType, tableName string, tableID uint64) error {
	result := DB.Where(
		"user_id = ? AND type = ? AND table_name = ? AND table_id = ?",
		userID, t, tableName, tableID,
	).Delete(&Notification{})
	return result.Error
}

// RemoveTypedNotification clears all notifications of a certain type for the user.
func RemoveTypedNotification(userID uint64, t NotificationType) error {
	result := DB.Where(
		"user_id = ? AND type = ?",
		userID, t,
	).Delete(&Notification{})
	return result.Error
}

// RemoveSpecificNotificationAboutUser to remove a specific table_name/id notification about a user,
// e.g. when removing a like on a photo.
func RemoveSpecificNotificationAboutUser(userID, aboutUserID uint64, t NotificationType, tableName string, tableID uint64) error {
	result := DB.Where(
		"user_id = ? AND about_user_id = ? AND type = ? AND table_name = ? AND table_id = ?",
		userID, aboutUserID, t, tableName, tableID,
	).Delete(&Notification{})
	return result.Error
}

// RemoveSpecificNotificationBulk can remove notifications about several TableIDs of the same type,
// e.g. to bulk remove new private photo upload notifications.
func RemoveSpecificNotificationBulk(users []*User, t NotificationType, tableName string, tableIDs []uint64) error {
	var userIDs = []uint64{}
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}

	if len(userIDs) == 0 {
		// Nothing to do.
		return errors.New("no user IDs given")
	}

	result := DB.Where(
		"user_id IN ? AND type = ? AND table_name = ? AND table_id IN ?",
		userIDs, t, tableName, tableIDs,
	).Delete(&Notification{})
	return result.Error
}

/*
RemoveCommentNotification removes notifications where a comment's body was copied to, e.g. when the user deletes the comment.

This removes notifications where:

1. The AboutUser, TableName and TableID match the Comment, with Type of Commented, AlsoCommented or AlsoPosted.
2. Any notification about the Comment itself (TableName="comments", TableID=Comment.ID)
*/
func RemoveCommentNotification(comment *Comment) error {
	result := DB.Exec(
		`
			DELETE FROM notifications
			WHERE (
				-- Comment, AlsoCommented notifications
				about_user_id = ?
				AND table_name = ? AND table_id = ?
				AND type IN ?
			) OR (
				-- Notifs for the Comment itself (e.g. likes)
				table_name = ? AND table_id = ?
			)
		`,
		comment.UserID,
		comment.TableName,
		comment.TableID,
		[]NotificationType{
			NotificationComment,
			NotificationAlsoCommented,
		},
		"comments", comment.ID,
	)
	return result.Error
}

// MarkNotificationsRead sets all a user's notifications to read.
func MarkNotificationsRead(user *User) error {
	return DB.Model(&Notification{}).Where(
		"user_id = ? AND read IS NOT TRUE",
		user.ID,
	).Update("read", true).Error
}

// MarkSpecificNotificationsRead updates a set of notification IDs to mark as read for the current user.
func MarkSpecificNotificationsRead(user *User, IDs []uint64) error {
	return DB.Model(&Notification{}).Where(
		"user_id = ? AND id IN ?",
		user.ID,
		IDs,
	).Update("read", true).Error
}

// ClearAllNotifications removes a user's entire notification table.
func ClearAllNotifications(user *User) error {
	return DB.Where(
		"user_id = ?", user.ID,
	).Delete(&Notification{}).Error
}

// ClearSpecificNotifications removes a set of notification IDs for the user.
func ClearSpecificNotifications(user *User, IDs []uint64) error {
	return DB.Model(&Notification{}).Where(
		"user_id = ? AND id IN ?",
		user.ID,
		IDs,
	).Delete(&Notification{}).Error
}

// CountUnreadNotifications gets the count of unread Notifications for a user.
func CountUnreadNotifications(user *User) (int64, error) {
	var (
		where = []string{
			"user_id = ? AND read = ?",
		}
		placeholders = []any{
			user.ID, false,
		}
	)

	// Blocking user IDs?
	bw, bp := BlockedUserSubquery("about_user_id", user.ID)
	where = append(where, bw)
	placeholders = append(placeholders, bp...)

	// Don't show messages from banned or disabled accounts.
	where = append(where, `
		EXISTS (
			SELECT 1
			FROM users
			WHERE users.id = notifications.about_user_id
			AND users.status = 'active'
		)
	`)

	query := DB.Where(
		strings.Join(where, " AND "),
		placeholders...,
	)

	var count int64
	result := query.Model(&Notification{}).Count(&count)
	return count, result.Error
}

// PaginateNotifications returns the user's notifications.
func PaginateNotifications(user *User, filters NotificationFilter, pageSize int, beforeID uint64) ([]*Notification, error) {
	var (
		ns    = []*Notification{}
		where = []string{
			"user_id = ?",
		}
		placeholders = []any{
			user.ID,
		}
		orderBy = "read, created_at desc"
	)

	// Pagination.
	if beforeID > 0 {
		where = append(where, "id < ?")
		placeholders = append(placeholders, beforeID)
	}

	// Blocking user IDs?
	bw, bp := BlockedUserSubquery("about_user_id", user.ID)
	where = append(where, bw)
	placeholders = append(placeholders, bp...)

	// Don't show notifications from banned or disabled accounts.
	where = append(where, `
		EXISTS (
			SELECT 1
			FROM users
			WHERE users.id = notifications.about_user_id
			AND users.status = 'active'
		)
	`)

	// Mix in notification type filters?
	if w, ph, ok := filters.Query(); ok {
		where = append(where, w)
		placeholders = append(placeholders, ph)
	}

	query := (&Notification{}).Preload().Where(
		strings.Join(where, " AND "),
		placeholders...,
	).Order(
		orderBy,
	)

	result := query.Limit(pageSize).Find(&ns)
	return ns, result.Error
}

// FilterPhotoUploadNotificationUserIDs will narrow a set of UserIDs who would be notified about
// a new photo/video upload to respect each user's preference for notification opt-outs.
//
// It is assumed that userIDs are already narrowed down to Friends of the current user.
func FilterPhotoUploadNotificationUserIDs(currentUser *User, isExplicit, isPrivate bool, userIDs []uint64) []uint64 {
	var (
		result = []uint64{}

		// Collect notification opt-out profile fields and map them by user ID for easy lookup.
		prefs    = []*ProfileField{} // Global Notification preferences
		mapPrefs = map[uint64]map[string]bool{}
	)
	if len(userIDs) == 0 {
		return userIDs
	}

	// Collect opt-out preferences for these users.
	r := DB.Model(&ProfileField{}).Where(
		"user_id IN ? AND name IN ?",
		userIDs, []string{
			config.NotificationOptOutFriendPhotos,   // all friends' photos
			config.NotificationOptOutPrivatePhotos,  // private photos from friends
			config.NotificationOptOutExplicitPhotos, // explicit photos
		},
	).Find(&prefs)
	if r.Error != nil {
		log.Error("FilterPhotoUploadNotificationUserIDs: couldn't collect user preferences: %s", r.Error)
	}

	// Map the preferences by user ID.
	for _, row := range prefs {
		if _, ok := mapPrefs[row.UserID]; !ok {
			mapPrefs[row.UserID] = map[string]bool{}
		}
		mapPrefs[row.UserID][row.Name] = row.Value == "true"
	}

	// Narrow the notification recipients based on photo property and their preferences.
	for _, userID := range userIDs {

		// Skip explicit photo notification?
		if isExplicit && mapPrefs[userID][config.NotificationOptOutExplicitPhotos] {
			continue
		}

		// Skip private photo notification?
		if isPrivate && mapPrefs[userID][config.NotificationOptOutPrivatePhotos] {
			continue
		}

		// Skip friend photo notifications?
		if mapPrefs[userID][config.NotificationOptOutFriendPhotos] {
			continue
		}

		// They get the notification.
		result = append(result, userID)
	}

	return result
}

// Save a notification.
func (n *Notification) Save() error {
	return DB.Save(n).Error
}

// Delete a notification.
func (n *Notification) Delete() error {
	return DB.Delete(n).Error
}

// NotificationBody can store remote tables mapped.
type NotificationBody struct {
	PhotoID   uint64
	VideoID   uint64
	ThreadID  uint64
	ForumID   uint64
	CommentID uint64
	BlogID    uint64
	Photo     *Photo
	Thread    *Thread
	Forum     *Forum
	Comment   *Comment
}

type NotificationMap map[uint64]*NotificationBody

// Get a notification's body from the map.
func (m NotificationMap) Get(id uint64) *NotificationBody {
	if body, ok := m[id]; ok {
		return body
	}
	return &NotificationBody{}
}

// MapNotifications loads associated assets, like Photos, mapped to their notification ID.
func MapNotifications(currentUser *User, ns []*Notification) NotificationMap {
	var (
		IDs    = []uint64{}
		result = NotificationMap{}
	)

	// Segregate the Notification IDs which are about various joinable tables.
	// so e.g. if no notification is about a Blog we don't need to run the useless SQL
	// query to join blog posts that won't exist.
	type ByTable struct {
		Photos  []uint64
		Threads []uint64
		Forums  []uint64
	}
	var byTable ByTable

	// Collect notification IDs.
	for _, row := range ns {
		IDs = append(IDs, row.ID)
		result[row.ID] = &NotificationBody{}

		// Segregate by referenced tables for deeper loading.
		switch row.TableName {
		case "photos":
			byTable.Photos = append(byTable.Photos, row.ID)
		case "threads":
			byTable.Threads = append(byTable.Threads, row.ID)
		case "forums":
			byTable.Forums = append(byTable.Forums, row.ID)
		}
	}

	result.mapNotificationPhotos(currentUser, byTable.Photos)
	result.mapNotificationThreads(byTable.Threads)
	result.mapNotificationForums(byTable.Forums)

	// NOTE: comment loading is not used - was added when trying to add "Like" buttons inside
	// your Comment notifications. But when a photo is commented on, the notification table_name=photos,
	// with the comment ID not so readily accessible.
	//
	// result.mapNotificationComments(IDs)

	return result
}

// Helper function of MapNotifications to eager load Photo attachments.
func (nm NotificationMap) mapNotificationPhotos(currentUser *User, IDs []uint64) {
	if len(IDs) == 0 {
		return
	}

	type scanner struct {
		PhotoID        uint64
		NotificationID uint64
	}
	var scan []scanner

	// Load all of these that have photos.
	err := DB.Table(
		"notifications",
	).Joins(
		"JOIN photos ON (notifications.table_name='photos' AND notifications.table_id=photos.id)",
	).Select(
		"photos.id AS photo_id",
		"notifications.id AS notification_id",
	).Where(
		`
			notifications.id IN ?
			AND (
				photos.user_id = ?
				OR photos.visibility = 'public'
				OR (
					photos.visibility = 'friends'
					AND EXISTS (
						SELECT 1
						FROM friends
						WHERE source_user_id = notifications.about_user_id
						AND target_user_id = ?
						AND approved IS TRUE
					)
				)
				OR (
					photos.visibility = 'private'
					AND EXISTS (
						SELECT 1
						FROM private_photos
						WHERE source_user_id = notifications.about_user_id
						AND target_user_id = ?
					)
				)
			)
		`,
		IDs, currentUser.ID, currentUser.ID, currentUser.ID,
	).Scan(&scan)
	if err.Error != nil {
		log.Error("Couldn't select photo IDs for notifications: %s", err.Error)
	}

	// Collect and load all the photos by ID.
	var photoIDs = []uint64{}
	for _, row := range scan {
		// Store the photo ID in the result now.
		nm[row.NotificationID].PhotoID = row.PhotoID
		photoIDs = append(photoIDs, row.PhotoID)
	}

	// Load the photos.
	if len(photoIDs) > 0 {
		if photos, err := GetPhotos(photoIDs); err != nil {
			log.Error("Couldn't load photo IDs for notifications: %s", err)
		} else {
			// Marry them to their notification IDs.
			for _, body := range nm {
				if photo, ok := photos[body.PhotoID]; ok {
					body.Photo = photo
				}
			}
		}
	}
}

// Helper function of MapNotifications to eager load Thread attachments.
func (nm NotificationMap) mapNotificationThreads(IDs []uint64) {
	if len(IDs) == 0 {
		return
	}

	type scanner struct {
		ThreadID       uint64
		NotificationID uint64
	}
	var scan []scanner

	// Load all of these that have threads.
	err := DB.Table(
		"notifications",
	).Joins(
		"JOIN threads ON (notifications.table_name='threads' AND notifications.table_id=threads.id)",
	).Select(
		"threads.id AS thread_id",
		"notifications.id AS notification_id",
	).Where(
		"notifications.id IN ?",
		IDs,
	).Scan(&scan)
	if err.Error != nil {
		log.Error("Couldn't select thread IDs for notifications: %s", err.Error)
	}

	// Collect and load all the threads by ID.
	var threadIDs = []uint64{}
	for _, row := range scan {
		// Store the thread ID in the result now.
		nm[row.NotificationID].ThreadID = row.ThreadID
		threadIDs = append(threadIDs, row.ThreadID)
	}

	// Load the threads.
	if len(threadIDs) > 0 {
		if threads, err := GetThreads(threadIDs); err != nil {
			log.Error("Couldn't load thread IDs for notifications: %s", err)
		} else {
			// Marry them to their notification IDs.
			for _, body := range nm {
				if thread, ok := threads[body.ThreadID]; ok {
					body.Thread = thread
				}
			}
		}
	}
}

// Helper function of MapNotifications to eager load Forum attachments.
func (nm NotificationMap) mapNotificationForums(IDs []uint64) {
	if len(IDs) == 0 {
		return
	}

	type scanner struct {
		ForumID        uint64
		NotificationID uint64
	}
	var scan []scanner

	// Load all of these that have forums.
	err := DB.Table(
		"notifications",
	).Joins(
		"JOIN forums ON (notifications.table_name='forums' AND notifications.table_id=forums.id)",
	).Select(
		"forums.id AS forum_id",
		"notifications.id AS notification_id",
	).Where(
		"notifications.id IN ?",
		IDs,
	).Scan(&scan)
	if err.Error != nil {
		log.Error("Couldn't select forum IDs for notifications: %s", err.Error)
	}

	// Collect and load all the forums by ID.
	var forumIDs = []uint64{}
	for _, row := range scan {
		// Store the forum ID in the result now.
		nm[row.NotificationID].ForumID = row.ForumID
		forumIDs = append(forumIDs, row.ForumID)
	}

	// Load the forums.
	if len(forumIDs) > 0 {
		if forums, err := GetForums(forumIDs); err != nil {
			log.Error("Couldn't load forum IDs for notifications: %s", err)
		} else {
			// Marry them to their notification IDs.
			for _, body := range nm {
				if forum, ok := forums[body.ForumID]; ok {
					body.Forum = forum
				}
			}
		}
	}
}

// Helper function of MapNotifications to eager load Blog attachments.
func (nm NotificationMap) mapNotificationBlogs(IDs []uint64) {
	if len(IDs) == 0 {
		return
	}

	type scanner struct {
		BlogID         uint64
		NotificationID uint64
	}
	var scan []scanner

	// Load all of these that have blogs.
	err := DB.Table(
		"notifications",
	).Joins(
		"JOIN blogs ON (notifications.table_name='blogs' AND notifications.table_id=blogs.id)",
	).Select(
		"blogs.id AS blog_id",
		"notifications.id AS notification_id",
	).Where(
		"notifications.id IN ?",
		IDs,
	).Scan(&scan)
	if err.Error != nil {
		log.Error("Couldn't select blog IDs for notifications: %s", err.Error)
	}

	// Collect and load all the blogs by ID.
	var blogIDs = []uint64{}
	for _, row := range scan {
		// Store the blog ID in the result now.
		nm[row.NotificationID].BlogID = row.BlogID
		blogIDs = append(blogIDs, row.BlogID)
	}
}

// Helper function of MapNotifications to eager load Comment attachments.
func (nm NotificationMap) mapNotificationComments(IDs []uint64) {
	type scanner struct {
		CommentID      uint64
		NotificationID uint64
	}
	var scan []scanner

	// Load all of these that have comments.
	err := DB.Table(
		"notifications",
	).Joins(
		"JOIN comments ON (notifications.table_name='comments' AND notifications.table_id=comments.id)",
	).Select(
		"comments.id AS comment_id",
		"notifications.id AS notification_id",
	).Where(
		"notifications.id IN ?",
		IDs,
	).Scan(&scan)
	if err.Error != nil {
		log.Error("Couldn't select comment IDs for notifications: %s", err.Error)
	}

	// Collect and load all the comments by ID.
	var commentIDs = []uint64{}
	for _, row := range scan {
		// Store the comment ID in the result now.
		nm[row.NotificationID].CommentID = row.CommentID
		commentIDs = append(commentIDs, row.CommentID)
	}

	// Load the comments.
	if len(commentIDs) > 0 {
		if comments, err := GetComments(commentIDs); err != nil {
			log.Error("Couldn't load comment IDs for notifications: %s", err)
		} else {
			// Marry them to their notification IDs.
			for _, body := range nm {
				if comment, ok := comments[body.CommentID]; ok {
					body.Comment = comment
				}
			}
		}
	}
}
