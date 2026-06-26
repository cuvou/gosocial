package models

import "time"

/*
UsageStatistic holds basic analytics points for things like daily/monthly active user counts.

Generally, there will be one UserStatistic row for each combination of a UserID and Type for
each calendar day of the year. Type names may be like "dau" to log daily logins (Daily Active User),
or "chat" to log daily chat room users.

If a user logs in multiple times in the same day, their existing UsageStatistic for that day
is reused and the Counter is incremented. So if a user joins chat 3 times on the same day, there
will be a single row for that date for that user, but with a Counter of 3 in that case.

This makes it easier to query for aggregate reports on daily/monthly active users since each
row/event type combo only appears once per user per day.
*/
type UsageStatistic struct {
	ID        uint64 `gorm:"primaryKey"`
	UserID    uint64 `gorm:"uniqueIndex:idx_usage_statistics"`
	Type      string `gorm:"uniqueIndex:idx_usage_statistics"`
	Date      string `gorm:"uniqueIndex:idx_usage_statistics"` // unique days, yyyy-mm-dd format.
	Counter   uint64
	CreatedAt time.Time `gorm:"index"` // full timestamps
	UpdatedAt time.Time `gorm:"index"`
}

// Options for UsageStatistic Type values.
const (
	UsageStatisticDailyVisit  = "dau"     // daily active user counter
	UsageStatisticChatEntry   = "chat"    // daily chat room users
	UsageStatisticForumUser   = "forum"   // daily forum users (when they open a thread)
	UsageStatisticGalleryUser = "gallery" // daily Site Gallery user (when viewing the site gallery)
	UsageStatisticBlogUser    = "blog"    // daily Blog visitors
	UsageStatisticVideoUser   = "video"   // daily Video Gallery user
)

// LogDailyActiveUser will ping a UserStatistic for the current user to mark them present for the day.
func LogDailyActiveUser(user *User) error {
	var (
		date   = time.Now().Format(time.DateOnly)
		_, err = IncrementUsageStatistic(user, UsageStatisticDailyVisit, date)
	)
	return err
}

// LogDailyChatUser will ping a UserStatistic for the current user to mark them as having used the chat room today.
func LogDailyChatUser(user *User) error {
	var (
		date   = time.Now().Format(time.DateOnly)
		_, err = IncrementUsageStatistic(user, UsageStatisticChatEntry, date)
	)
	return err
}

// LogDailyForumUser will ping a UserStatistic for the current user to mark them as having used the forums today.
func LogDailyForumUser(user *User) error {
	var (
		date   = time.Now().Format(time.DateOnly)
		_, err = IncrementUsageStatistic(user, UsageStatisticForumUser, date)
	)
	return err
}

// LogDailyGalleryUser will ping a UserStatistic for the current user to mark them as having used the site gallery today.
func LogDailyGalleryUser(user *User) error {
	var (
		date   = time.Now().Format(time.DateOnly)
		_, err = IncrementUsageStatistic(user, UsageStatisticGalleryUser, date)
	)
	return err
}

// LogDailyBlogUser will ping a UserStatistic for the current user to mark them as having used the Blogs feature today.
func LogDailyBlogUser(user *User) error {
	var (
		date   = time.Now().Format(time.DateOnly)
		_, err = IncrementUsageStatistic(user, UsageStatisticBlogUser, date)
	)
	return err
}

// LogDailyVideoUser will ping a UserStatistic for the current user to mark them as having used the Video Gallery feature today.
func LogDailyVideoUser(user *User) error {
	var (
		date   = time.Now().Format(time.DateOnly)
		_, err = IncrementUsageStatistic(user, UsageStatisticVideoUser, date)
	)
	return err
}

// GetUsageStatistic looks up a user statistic.
func GetUsageStatistic(user *User, statType, date string) (*UsageStatistic, error) {
	var (
		result = &UsageStatistic{}
		res    = DB.Model(&UsageStatistic{}).Where(
			"user_id = ? AND type = ? AND date = ?",
			user.ID, statType, date,
		).First(&result)
	)
	return result, res.Error
}

// IncrementUsageStatistic finds or creates a UserStatistic type and increments the counter.
func IncrementUsageStatistic(user *User, statType, date string) (*UsageStatistic, error) {
	user.muStatistic.Lock()
	defer user.muStatistic.Unlock()

	// Is there an existing row?
	stat, err := GetUsageStatistic(user, statType, date)
	if err != nil {
		stat = &UsageStatistic{
			UserID:  user.ID,
			Type:    statType,
			Counter: 0,
			Date:    date,
		}
	}

	// Update and save it.
	stat.Counter++
	err = DB.Save(stat).Error
	return stat, err
}
