package models

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/markdown"
	"github.com/cuvou/gosocial/pkg/utility"
	"gorm.io/gorm/clause"
)

// ProfileField table for arbitrary user profile settings.
type ProfileField struct {
	ID        uint64 `gorm:"primaryKey"`
	UserID    uint64 `gorm:"uniqueIndex:idx_profile_field"`
	Name      string `gorm:"uniqueIndex:idx_profile_field"`
	Value     string `gorm:"index"`
	LongValue string // e.g. profile essays
	CreatedAt time.Time
	UpdatedAt time.Time
}

// PreloadUserProfileFields loads profile fields for a batch of users at once.
func PreloadUserProfileFields(users []*User) error {
	// Collect user IDs who are missing their caches.
	var (
		userIDs  = []uint64{}
		mapUsers = map[uint64]*User{}
	)
	for _, user := range users {
		if user.cacheProfileFields == nil {
			userIDs = append(userIDs, user.ID)
			mapUsers[user.ID] = user
		}
	}

	// Any to load?
	if len(userIDs) == 0 {
		return nil
	}

	// Load all their profile fields.
	var (
		pf  = []*ProfileField{}
		res = DB.Model(&ProfileField{}).Where(
			"user_id IN ?",
			userIDs,
		).Find(&pf)
	)
	if res.Error != nil {
		return fmt.Errorf("PreloaduserProfileFields: %w", res.Error)
	}

	// Populate their caches.
	for _, row := range pf {
		if user, ok := mapUsers[row.UserID]; ok {
			if user.cacheProfileFields == nil {
				user.cacheProfileFields = map[string]*ProfileField{}
			}
			user.cacheProfileFields[row.Name] = row
		}
	}

	return nil
}

// ProfileTabCount stores structured counts for the profile page tabs.
type ProfileTabCount struct {
	Photos  int64
	Friends int64
}

// GetProfileTabCounts queries efficiently for the count badges on a user's profile page tabs.
func (u *User) GetProfileTabCounts(currentUser *User) (ProfileTabCount, error) {
	var (
		result            = ProfileTabCount{}
		isSelf            = u.ID == currentUser.ID
		areFriends        = isSelf || AreFriends(u.ID, currentUser.ID)
		isPrivateUnlocked = isSelf || IsPrivateUnlocked(u.ID, currentUser.ID)
		photoVisibility   = []PhotoVisibility{
			PhotoPublic,
		}
	)

	if areFriends {
		photoVisibility = append(photoVisibility, PhotoFriends)
	}

	if isPrivateUnlocked {
		photoVisibility = append(photoVisibility, PhotoPrivate)
	}

	type record struct {
		MetricName  string
		MetricCount int64
	}
	var (
		records []record
		res     = DB.Raw(
			`
				-- Photo count
				WITH subquery_photos AS (
					SELECT COUNT(*) AS cnt
					FROM photos
					WHERE user_id = ?
					AND visibility IN ?
				),

				-- Friend count
				subquery_friends AS (
					SELECT COUNT(*) AS cnt
					FROM friends
					WHERE target_user_id = ?
					AND approved IS TRUE
					AND EXISTS (
						SELECT 1
						FROM users
						WHERE users.id = friends.source_user_id
						AND users.status = 'active'
					)
				)

				-- Combine the data.
				SELECT
					'photos' AS metric_name,
					cnt AS metric_count
				FROM subquery_photos

				UNION ALL

				SELECT
					'friends' AS metric_name,
					cnt AS metric_count
				FROM subquery_friends
			`,
			u.ID, photoVisibility,
			u.ID,
		).Scan(&records)
	)

	if res.Error != nil {
		return result, res.Error
	}

	for _, row := range records {
		switch row.MetricName {
		case "photos":
			result.Photos = row.MetricCount
		case "friends":
			result.Friends = row.MetricCount
		default:
			return result, fmt.Errorf("unknown metric name: %s", row.MetricName)
		}
	}

	return result, nil
}

// LoadProfileFields loads and caches all profile fields on the user.
func (u *User) LoadProfileFields() error {
	var (
		pf  = []*ProfileField{}
		res = DB.Model(&ProfileField{}).Where(
			"user_id = ?",
			u.ID,
		).Find(&pf)
	)
	if res.Error != nil {
		return fmt.Errorf("LoadProfileFields: %w", res.Error)
	}

	u.cacheProfileFields = map[string]*ProfileField{}
	for _, row := range pf {
		u.cacheProfileFields[row.Name] = row
	}

	return nil
}

// GetProfileField returns the value of a profile field or blank string.
func (u *User) GetProfileField(name string) string {
	if u.cacheProfileFields == nil {
		if err := u.LoadProfileFields(); err != nil {
			log.Error("GetProfileField(%s): %w", name, err)
			return ""
		}
	}

	if field, ok := u.cacheProfileFields[name]; ok {
		return field.Value
	}

	return ""
}

// GetProfileFieldOr returns a default string (like "n/a") if the profile field is not set.
func (u *User) GetProfileFieldOr(name, or string) string {
	if value := u.GetProfileField(name); value != "" {
		return value
	}
	return or
}

// GetDisplayAge returns the user's age dependent on their hide-my-age setting.
func (u *User) GetDisplayAge() string {
	if !u.Birthdate.IsZero() && u.GetProfileField("hide_age") != "true" {
		return fmt.Sprintf("%dyo", utility.Age(u.Birthdate))
	}

	return "n/a"
}

// StatusMessageExpiresAt returns a pretty duration of when the user's status message will expire.
func (u *User) StatusMessageExpiresAt() string {
	var (
		expiresStr    = u.GetProfileField("headline_expires")
		unixTime, err = strconv.ParseInt(expiresStr, 10, 64)
	)

	if expiresStr == "" || err != nil || unixTime == 0 {
		return ""
	}

	then := time.Unix(unixTime, 0)
	return utility.FormatDurationFloatingCoarse(time.Since(then))
}

// StatusMessageExpiresHours returns the current headline's expiration at and casts it to an integer hour amount.
//
// This is for editing an existing status message that will expire: so the Expires dropdown can show the current
// time left as the default value unless the user changes it.
func (u *User) StatusMessageExpiresHours() int {
	var (
		expiresStr    = u.GetProfileField("headline_expires")
		unixTime, err = strconv.ParseInt(expiresStr, 10, 64)
	)

	if expiresStr == "" || err != nil || unixTime == 0 {
		return 0
	}

	var (
		// Pad 30 minutes onto it, so e.g. if the user had JUST posted a 2-hour expiration and
		// they edit it again, it will round up to 2 instead of down to 1 hour.
		then  = time.Unix(unixTime+60*30, 0)
		since = time.Since(then)
		hours = math.Abs(since.Hours())
	)
	return int(hours)
}

// SetProfileField sets or creates a named profile field.
func (u *User) SetProfileField(name, value string) {
	if u.cacheProfileFields == nil {
		if err := u.LoadProfileFields(); err != nil {
			log.Error("GetProfileField(%s): %w", name, err)
			return
		}
	}

	// Check if it already exists.
	if field, ok := u.cacheProfileFields[name]; ok {
		log.Debug("\tFound existing field! To set %s=%s", name, value)
		if field.Value != value {
			field.Value = value
			DB.Save(field)
		}
		return
	}

	log.Debug("User(%s): append ProfileField %s", u.Username, name)
	pf := &ProfileField{
		UserID: u.ID,
		Name:   name,
		Value:  value,
	}
	res := DB.Model(&ProfileField{}).Clauses(
		clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "name"},
			},
			UpdateAll: true,
		},
	).Create(pf)

	if res.Error != nil {
		log.Error("SetProfileField(%s, %s): Upsert error: %w", u.Username, name, res.Error)
	}

	// Update their profile field cache.
	if pf.ID > 0 {
		u.cacheProfileFields[name] = pf
	} else {
		// Shouldn't happen, reload the profile fields if it does.
		if err := u.LoadProfileFields(); err != nil {
			log.Error("Reload profile fields cache: %s", err)
		}
	}
}

// SetLongProfileField stores a long profile field (such as an essay) in an index-optimized way.
//
// This enables longer profile texts, e.g. with lots of HTML markup or color-gradient BBCode tags,
// to be stored without the text length overflowing the index limit on the Value column.
//
// This is intended for Markdown/BBCode formatted profile fields.
//
// The value will be rendered to HTML and then stripped of all the HTML tags (reducing it to
// just the visible text portions, to aid the search index) and that will be stored as the Value.
// Meanwhile, the full raw input string will be placed in the LongValue column.
func (u *User) SetLongProfileField(name, longValue string) {
	var (
		// Render to HTML and strip down the short value.
		html  = markdown.Render(longValue)
		short = markdown.StripHTML(html)
	)

	if len(short) > config.MaxDatabaseStringIndexSize {
		short = short[:config.MaxDatabaseStringIndexSize]
	}

	// Easy upsert.
	pf := &ProfileField{
		UserID:    u.ID,
		Name:      name,
		Value:     short,
		LongValue: longValue,
	}
	res := DB.Model(&ProfileField{}).Clauses(
		clause.OnConflict{
			Columns: []clause.Column{
				{Name: "user_id"},
				{Name: "name"},
			},
			UpdateAll: true,
		},
	).Create(pf)

	if res.Error != nil {
		log.Error("SetProfileField(%s, %s): Upsert error: %w", u.Username, name, res.Error)
	}

	// Update the ProfileFields cache on the user.
	if u.cacheProfileFields != nil {
		u.cacheProfileFields[name] = pf
	}
}

// GetLongProfileField returns a profile field which may have a LongValue, e.g. for profile essays.
//
// If the ProfileField has no LongValue defined, its Value is returned instead like GetProfileField.
//
// This supports a graceful migration for legacy profiles who had only Values stored for their essays
// but have not saved an update to their profile to have the LongValue separated out yet.
func (u *User) GetLongProfileField(name string) string {
	if u.cacheProfileFields == nil {
		if err := u.LoadProfileFields(); err != nil {
			log.Error("GetProfileField(%s): %w", name, err)
			return ""
		}
	}

	if field, ok := u.cacheProfileFields[name]; ok {
		// A LongValue is available?
		if field.LongValue != "" {
			return field.LongValue
		}
		return field.Value
	}

	return ""
}

// DeleteProfileField removes a stored profile field.
func (u *User) DeleteProfileField(name string) error {
	res := DB.Exec(
		"DELETE FROM profile_fields WHERE user_id=? AND name=?",
		u.ID, name,
	)
	return res.Error
}

// ProfileFieldIn checks if a substring is IN a profile field. Currently
// does a naive strings.Contains(), intended for the "here_for" field.
func (u *User) ProfileFieldIn(field, substr string) bool {
	value := u.GetProfileField(field)
	return strings.Contains(value, substr)
}
