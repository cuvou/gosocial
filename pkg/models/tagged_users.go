package models

import (
	"fmt"
	"sort"
	"time"

	"github.com/cuvou/gosocial/pkg/log"
	"gorm.io/gorm/clause"
)

// TaggedUser table tags people in content they appeared in.
type TaggedUser struct {
	UserID    uint64 `gorm:"uniqueIndex:idx_tagged_user"`
	TableName string `gorm:"uniqueIndex:idx_tagged_user"`
	TableID   uint64 `gorm:"uniqueIndex:idx_tagged_user"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TaggableUserTables is a map of table names that accept tags.
// Note: update goto_tagged_user.go when adding new tables!
var TaggableUserTables = map[string]any{
	"photos": nil,
}

// TagUser upserts a user tag related to e.g. a photo or video.
func TagUser(sourceUserID, userID uint64, tableName string, tableID uint64) error {
	if IsUserTagged(userID, tableName, tableID) {
		return nil
	}

	// Upsert the tag.
	var (
		tag = &TaggedUser{
			UserID:    userID,
			TableName: tableName,
			TableID:   tableID,
		}
		res = DB.Model(&TaggedUser{}).Clauses(
			clause.OnConflict{
				Columns: []clause.Column{
					{Name: "user_id"},
					{Name: "table_name"},
					{Name: "table_id"},
				},
				UpdateAll: true,
			},
		).Create(&tag)
	)

	// Notify the user.
	notif := &Notification{
		UserID:      userID,
		AboutUserID: &sourceUserID,
		Type:        NotificationTaggedUser,
		TableName:   tableName,
		TableID:     tableID,
		Link:        fmt.Sprintf("/go/tagged?table=%s&id=%d", tableName, tableID),
	}
	if err := CreateNotification(notif); err != nil {
		log.Error("TagUser: didn't create notification: %s", err)
	}

	return res.Error
}

// IsUserTagged returns whether the user is tagged in your content.
func IsUserTagged(userID uint64, tableName string, tableID uint64) bool {
	var count int64
	DB.Model(&TaggedUser{}).Where(
		"user_id = ? AND table_name = ? AND table_id = ?",
		userID, tableName, tableID,
	).Count(&count)
	return count > 0
}

// CountTaggedUsers returns how many users are tagged.
func CountTaggedUsers(tableName string, tableID uint64) int {
	var count int64
	DB.Model(&TaggedUser{}).Where(
		"table_name = ? AND table_id = ?",
		tableName, tableID,
	).Count(&count)
	return int(count)
}

// GetTaggedUsers returns a list of users tagged.
func GetTaggedUsers(currentUser *User, tableName string, tableID uint64) ([]*User, error) {
	var (
		tags    []*TaggedUser
		userIDs []uint64
		res     = DB.Model(&TaggedUser{}).Where(
			"table_name = ? AND table_id = ?",
			tableName, tableID,
		).Find(&tags)
	)
	if res.Error != nil {
		return nil, res.Error
	}

	for _, tag := range tags {
		userIDs = append(userIDs, tag.UserID)
	}

	users, err := GetUsers(currentUser, userIDs)
	if err != nil {
		return nil, err
	}

	// Sort the users by name.
	sort.Slice(users, func(i, j int) bool {
		return users[i].Username < users[j].Username
	})

	// Filter out blocked users.
	var result = []*User{}
	for _, user := range users {
		if user.UserRelationship.IsBlocked {
			continue
		}
		result = append(result, user)
	}

	return result, nil
}

// UntagUser removes a user tag.
func UntagUser(userID uint64, tableName string, tableID uint64) error {

	// Revoke notifications.
	if err := RemoveSpecificNotification(userID, NotificationTaggedUser, tableName, tableID); err != nil {
		log.Error("UntagUser: couldn't revoke notification: %s", err)
	}

	return DB.Exec(
		`
			DELETE FROM tagged_users
			WHERE user_id = ?
			AND table_name = ?
			AND table_id = ?
		`,
		userID, tableName, tableID,
	).Error
}
