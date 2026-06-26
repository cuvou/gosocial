// Package models handles the database.
package models

import "gorm.io/gorm"

// DB to be set by calling app (SQLite or Postgres connection).
var DB *gorm.DB

// AutoMigrate the schema.
func AutoMigrate() {

	DB.AutoMigrate(
		// User and user-generated data.
		// ✔ = models are cleaned up on DeleteUser()
		&AdminGroup{},       // ✔ admin_group_users
		&Block{},            // ✔
		&ChangeLog{},        // ✔
		&Comment{},          // ✔
		&CommentPhoto{},     // ✔
		&Feedback{},         // ✔
		&ForumMembership{},  // ✔
		&Friend{},           // ✔
		&IPAddress{},        // ✔
		&Like{},             // ✔
		&Message{},          // ✔
		&Notification{},     // ✔
		&ProfileField{},     // ✔
		&Photo{},            // ✔
		&PollVote{},         // keep their vote on polls
		&Poll{},             // vacuum script cleans up orphaned polls
		&PrivatePhoto{},     // ✔
		&PushNotification{}, // ✔
		&Subscription{},     // ✔
		&Thread{},           // ✔
		&TwoFactor{},        // ✔
		&UsageStatistic{},   // ✔
		&User{},             // ✔
		&UserLocation{},     // ✔
		&PrivacySetting{},   // ✔
		&ProfileTheme{},     // ✔
		&LoginSession{},     // ✔
		&Follow{},           // ✔
		&TaggedUser{},       // ✔

		// Non-user or persistent data.
		&AdminScope{},
		&Forum{},

		// Vendor/misc data.
		&WorldCities{},

		// BareRTC shared tables.
		// Note: BareRTC will delete these on account deletion.
		&DirectMessage{},
	)
}
