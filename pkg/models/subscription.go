package models

import (
	"time"

	"github.com/cuvou/gosocial/pkg/log"
)

// Subscription table - for notifications. You comment on someone's post, you get subscribed
// to other comments added to the post (unless you opt off).
type Subscription struct {
	ID         uint64 `gorm:"primaryKey"`
	UserID     uint64 `gorm:"index"` // who it belongs to
	Subscribed bool   `gorm:"index"`
	TableName  string `gorm:"index"` // on which of your tables (photos, comments, ...)
	TableID    uint64 `gorm:"index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// GetSubscription looks for an existing subscription or returns error if not found.
func GetSubscription(user *User, tableName string, tableID uint64) (*Subscription, error) {
	var s *Subscription
	result := DB.Model(s).Where(
		"user_id = ? AND table_name = ? AND table_id = ?",
		user.ID, tableName, tableID,
	).First(&s)
	return s, result.Error
}

// CountSubscriptions counts how many comment threads the user is subscribed to.
func CountSubscriptions(user *User) int64 {
	var (
		count  int64
		result = DB.Model(&Subscription{}).Where(
			"user_id = ? AND subscribed IS TRUE",
			user.ID,
		).Count(&count)
	)
	if result.Error != nil {
		log.Error("Error in CountSubscriptions(%s): %s", user.Username, result.Error)
	}
	return count
}

// UnsubscribeAllThreads removes subscription preferences for all comment threads.
func UnsubscribeAllThreads(user *User) error {
	return DB.Where(
		"user_id = ?",
		user.ID,
	).Delete(&Subscription{}).Error
}

// GetSubscribers returns all of the UserIDs that are subscribed to a thread.
func GetSubscribers(tableName string, tableID uint64) []uint64 {
	var userIDs = []uint64{}
	result := DB.Table(
		"subscriptions",
	).Select(
		"user_id",
	).Where(
		"table_name = ? AND table_id = ? AND subscribed IS TRUE",
		tableName, tableID,
	).Scan(&userIDs)

	if result.Error != nil {
		log.Error("GetSubscribers(%s, %d): couldn't get user IDs: %s", tableName, tableID, result.Error)
	}

	return userIDs
}

// IsSubscribed checks whether a user is currently subscribed (and notified) to a thing.
// Returns whether the row exists, and whether the user is to be notified (false if opted out).
func IsSubscribed(user *User, tableName string, tableID uint64) (exists bool, notified bool) {
	if sub, err := GetSubscription(user, tableName, tableID); err != nil {
		return false, false
	} else {
		return true, sub.Subscribed
	}
}

// SubscribeTo creates a subscription to a thing (comment thread) to be notified of future activity on.
//
// If a Subscription row already exists, it is NOT modified. So if a user has expressly opted out of a
// comment thread, they do not get re-subscribed when they comment on it again.
func SubscribeTo(user *User, tableName string, tableID uint64) (*Subscription, error) {
	// Is there already a subscription row?
	if sub, err := GetSubscription(user, tableName, tableID); err == nil {
		return sub, err
	}

	// Create the default subscription.
	sub := &Subscription{
		UserID:     user.ID,
		Subscribed: true,
		TableName:  tableName,
		TableID:    tableID,
	}
	result := DB.Create(sub)
	return sub, result.Error
}

// UnsubscribeTo will create an explicit opt-out Subscription only if no subscription exists.
//
// It is the inverse of SubscribeTo.
func UnsubscribeTo(user *User, tableName string, tableID uint64) (*Subscription, error) {
	// Is there already a subscription row?
	if sub, err := GetSubscription(user, tableName, tableID); err == nil {
		return sub, err
	}

	// Create the default subscription.
	sub := &Subscription{
		UserID:     user.ID,
		Subscribed: false,
		TableName:  tableName,
		TableID:    tableID,
	}
	result := DB.Create(sub)
	return sub, result.Error
}

// Save a subscription.
func (n *Subscription) Save() error {
	return DB.Save(n).Error
}
