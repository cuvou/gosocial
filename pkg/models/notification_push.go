package models

import (
	"time"

	"github.com/cuvou/gosocial/pkg/log"
)

// PushNotification table for Web Push subscriptions.
type PushNotification struct {
	ID           uint64 `gorm:"primaryKey"`
	UserID       uint64 `gorm:"index"`
	Subscription string `gorm:"index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// RegisterPushNotification stores a registration for the user.
func RegisterPushNotification(user *User, subscription string) (*PushNotification, error) {
	// Check for an existing registration.
	pn, err := GetPushNotificationFromSubscription(user, subscription)
	if err == nil {
		return pn, nil
	}

	// Create it.
	pn = &PushNotification{
		UserID:       user.ID,
		Subscription: subscription,
	}
	result := DB.Create(pn)
	return pn, result.Error
}

// GetPushNotificationFromSubscription checks for an existing subscription.
func GetPushNotificationFromSubscription(user *User, subscription string) (*PushNotification, error) {
	var (
		pn     *PushNotification
		result = DB.Model(&PushNotification{}).Where(
			"user_id = ? AND subscription = ?",
			user.ID, subscription,
		).First(&pn)
	)
	return pn, result.Error
}

// CountPushNotificationSubscriptions returns how many subscriptions the user has for push.
func CountPushNotificationSubscriptions(user *User) int64 {
	var count int64
	result := DB.Where(
		"user_id = ?",
		user.ID,
	).Model(&PushNotification{}).Count(&count)
	if result.Error != nil {
		log.Error("CountPushNotificationSubscriptions(%d): %s", user.ID, result.Error)
	}
	return count
}

// GetPushNotificationSubscriptions returns all subscriptions for a user.
func GetPushNotificationSubscriptions(user *User) ([]*PushNotification, error) {
	var (
		pn     = []*PushNotification{}
		result = DB.Model(&PushNotification{}).Where("user_id = ?", user.ID).Scan(&pn)
	)
	return pn, result.Error
}

// DeletePushNotifications scrubs data for deleting a user.
func DeletePushNotificationSubscriptions(user *User) error {
	result := DB.Where(
		"user_id = ?",
		user.ID,
	).Delete(&PushNotification{})
	return result.Error
}

// DeletePushNotification removes a single subscription from the database.
func DeletePushNotification(user *User, subscription string) error {
	result := DB.Where(
		"user_id = ? AND subscription = ?",
		user.ID, subscription,
	).Delete(&PushNotification{})
	return result.Error
}
