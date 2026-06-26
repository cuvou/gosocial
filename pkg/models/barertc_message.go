package models

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/log"
	"gorm.io/gorm"
)

// DirectMessage table belongs to the chat room (BareRTC).
//
// This model should be kept in sync with BareRTC's DirectMessage model.
// On gosocial, the chat room will share the Postgres DB with the main
// website and it will manage this table. But from the gosocial side, we
// can include this table for user data exports and deep deletions.
type DirectMessage struct {
	MessageID int64  `gorm:"primaryKey"`
	ChannelID string `gorm:"index"`
	Username  string `gorm:"index"`
	Message   string
	Timestamp int64          // deprecated
	CreatedAt time.Time      `gorm:"index"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// BareRTCDirectMessageChannelName returns the sorted channel name for the DM
// thread between two usernames.
func BareRTCDirectMessageChannelName(usernames []string) string {
	// Ensure each username has the @ prefix.
	for i, user := range usernames {
		if !strings.HasPrefix(user, "@") {
			usernames[i] = "@" + user
		}
	}
	sort.Strings(usernames)
	return strings.Join(usernames, ":")
}

// PaginateBareRTCDirectMessages loads messages from the chat room.
func PaginateBareRTCDirectMessages(channelID string, pager *Pagination) ([]*DirectMessage, error) {
	var (
		dm = []*DirectMessage{}
	)

	query := DB.Unscoped().Where(
		"channel_id = ?",
		channelID,
	).Order(pager.Sort)

	query.Model(&DirectMessage{}).Count(&pager.Total)
	result := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&dm)
	return dm, result.Error
}

// GetBareRTCMessageInsights returns detailed insights about Direct Message behavior of a user.
func GetBareRTCMessageInsights(user *User) (*MessageInsight, error) {
	var result = &MessageInsight{
		Items: []*MessageInsightItem{},
	}

	type record struct {
		ChannelID     string
		MessageCount  int64
		LastMessageAt time.Time
	}

	var (
		likes = []string{
			fmt.Sprintf("@%s:%%", user.Username),
			fmt.Sprintf("%%:@%s", user.Username),
		}
		rows []record
		res  = DB.Raw(`
			SELECT
				DISTINCT(channel_id) AS channel_id,
				COUNT(*) AS message_count,
				MAX(created_at) AS last_message_at
			FROM direct_messages
			WHERE (
				channel_id LIKE ? OR channel_id LIKE ?
			)
			GROUP BY channel_id
			ORDER BY channel_id
		`, likes[0], likes[1]).Scan(&rows)
	)
	if res.Error != nil {
		return result, res.Error
	}

	// Process the message insights.
	var (
		usernames          = []string{}
		channelUsernameMap = map[string]string{}
	)
	for _, row := range rows {

		// Parse the other username out of the channel ID.
		var (
			username     string
			channelParts = strings.Split(row.ChannelID, ":")
		)
		for _, un := range channelParts {
			un = strings.TrimPrefix(un, "@")
			if un != user.Username {
				username = un
			}
		}

		if username == "" {
			username = user.Username
		}
		usernames = append(usernames, username)
		channelUsernameMap[row.ChannelID] = username
	}

	userMap, err := MapUsersByUsername(usernames)
	if err != nil {
		log.Error("GetBareRTCMessageInsights: MapUsersByUsername: %s", err)
	}

	for _, row := range rows {
		username := channelUsernameMap[row.ChannelID]
		if user, ok := userMap[username]; ok {
			result.Items = append(result.Items, &MessageInsightItem{
				ContactUser:   user,
				ContactUserID: user.ID,
				MessageCount:  row.MessageCount,
				LastMessageAt: row.LastMessageAt,
			})
		}
	}

	return result, nil
}
