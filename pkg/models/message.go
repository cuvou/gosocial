package models

import (
	"errors"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
)

// Message table.
type Message struct {
	ID           uint64 `gorm:"primaryKey"`
	SourceUserID uint64 `gorm:"index"`
	TargetUserID uint64 `gorm:"index"`
	Read         bool   `gorm:"index"`
	Message      string
	CreatedAt    time.Time
	UpdatedAt    time.Time

	// Private use properties.
	// On the Threads view, a bool saying if the current user has replied
	// to a message (has a newer timestamp than the message shown).
	HaveReplied bool `gorm:"-"`
}

// GetMessage by ID.
func GetMessage(id uint64) (*Message, error) {
	m := &Message{}
	result := DB.First(&m, id)
	return m, result.Error
}

// GetMessages for a user, e-mail style for the inbox or sent box view.
//
// The sent and all bools are mutually exclusive: default shows your Inbox, or you can
// opt to see the Sent tab or else the All tab.
func GetMessages(user *User, sent, all bool, search *Search, pager *Pagination) ([]*Message, error) {
	var (
		m            = []*Message{}
		where        = []string{}
		placeholders = []interface{}{}
	)

	if sent && all {
		return m, errors.New("sent and all tabs are mutually exclusive")
	}

	if sent {
		where = append(where, "source_user_id = ?")
		placeholders = append(placeholders, user.ID)

		// Not blocked users.
		bw, bp := BlockedUserSubquery("target_user_id", user.ID)
		where = append(where, bw)
		placeholders = append(placeholders, bp...)
	} else if all {
		where = append(where, "(source_user_id = ? OR target_user_id = ?)")
		placeholders = append(placeholders, user.ID, user.ID)

		// Not blocked users (either direction).
		if bw, bp := BlockedUserSubquery("target_user_id", user.ID); len(bw) > 0 {
			where = append(where, bw)
			placeholders = append(placeholders, bp...)
		}
		if bw, bp := BlockedUserSubquery("source_user_id", user.ID); len(bw) > 0 {
			where = append(where, bw)
			placeholders = append(placeholders, bp...)
		}
	} else {
		where = append(where, "target_user_id = ?")
		placeholders = append(placeholders, user.ID)

		// Not blocked users.
		bw, bp := BlockedUserSubquery("source_user_id", user.ID)
		where = append(where, bw)
		placeholders = append(placeholders, bp...)
	}

	// Search terms.
	if search != nil {
		for _, term := range search.Includes {
			var ilike = "%" + strings.ToLower(term) + "%"
			where = append(where, "message ILIKE ?")
			placeholders = append(placeholders, ilike)
		}
		for _, term := range search.Excludes {
			var ilike = "%" + strings.ToLower(term) + "%"
			where = append(where, "message NOT ILIKE ?")
			placeholders = append(placeholders, ilike)
		}
	}

	// Don't show messages from banned or disabled accounts.
	where = append(where, `
		NOT EXISTS (
			SELECT 1
			FROM users
			WHERE users.id IN (messages.target_user_id, messages.source_user_id)
			AND users.status <> 'active'
		)
	`)

	query := DB.Where(
		strings.Join(where, " AND "),
		placeholders...,
	).Order(pager.Sort)

	query.Model(&Message{}).Count(&pager.Total)
	result := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&m)

	// Map the HaveReplied boolean for these messages: collecting the partner user
	// IDs and the newest timestamp where we have a SourceUserID message to them.
	MapHaveRepliedMessages(user, m)

	return m, result.Error
}

// GetMessageThreads for a user: combined inbox/sent view grouped by username.
func GetMessageThreads(user *User, search *Search, pager *Pagination) ([]*Message, error) {
	var (
		m            = []*Message{}
		where        = []string{}
		placeholders = []interface{}{}
	)

	where = append(where, "target_user_id = ?")
	placeholders = append(placeholders, user.ID)

	// Blocking user IDs?
	bw, bp := BlockedUserSubquery("source_user_id", user.ID)
	where = append(where, bw)
	placeholders = append(placeholders, bp...)

	// Search terms.
	if search != nil {
		for _, term := range search.Includes {
			var ilike = "%" + strings.ToLower(term) + "%"
			where = append(where, "message ILIKE ?")
			placeholders = append(placeholders, ilike)
		}
		for _, term := range search.Excludes {
			var ilike = "%" + strings.ToLower(term) + "%"
			where = append(where, "message NOT ILIKE ?")
			placeholders = append(placeholders, ilike)
		}
	}

	// Don't show messages from banned or disabled accounts.
	where = append(where, `
		NOT EXISTS (
			SELECT 1
			FROM users
			WHERE users.id IN (messages.target_user_id, messages.source_user_id)
			AND users.status <> 'active'
		)
	`)

	type newest struct {
		ID           uint64
		SourceUserID uint64
		TargetUserID uint64
	}
	var scan []newest

	// Get the newest message IDs grouped by username for everyone we are chatting with.
	query := DB.Model(&Message{}).Select(
		"max(id) AS id",
		"source_user_id",
		"target_user_id",
	).Where(
		strings.Join(where, " AND "),
		placeholders...,
	).Group(
		"source_user_id, target_user_id",
	).Order("id desc").Scan(&scan)
	if query.Error != nil {
		return nil, query.Error
	}

	pager.Total = int64(len(scan))

	// Get the details from these message IDs.
	var messageIDs = []uint64{}
	for _, row := range scan {
		messageIDs = append(messageIDs, row.ID)
	}
	query = DB.Where(
		"id IN ?",
		messageIDs,
	).Order(pager.Sort)

	query.Model(&Message{}).Count(&pager.Total)
	result := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&m)

	// Map the HaveReplied boolean for these messages: collecting the partner user
	// IDs and the newest timestamp where we have a SourceUserID message to them.
	MapHaveRepliedMessages(user, m)

	return m, result.Error
}

// MapHaveRepliedMessages scans a set of received Messages and sets the HaveReplied boolean
// if the current user has a newer reply in the thread.
//
// This drives the "Threads" and "Inbox" views of the messages page, where the set of Messages
// are all sent to the target user (where the message TargetUserID is us). This function gathers
// all of the unique SourceUserIDs, and then checks for the latest messages sent by the current user
// to them to see whether to show the "Have Replied" badge.
func MapHaveRepliedMessages(currentUser *User, m []*Message) {
	// Map the HaveReplied boolean for these messages: collecting the partner user
	// IDs and the newest timestamp where we have a SourceUserID message to them.
	var (
		partnerIDs            = []uint64{}
		lastReplyToPartnerIDs = map[uint64]*Message{}
		newestReplies         = []*Message{}
	)
	for _, msg := range m {
		if msg.TargetUserID == currentUser.ID {
			partnerIDs = append(partnerIDs, msg.SourceUserID)
		}
	}
	if len(partnerIDs) > 0 {
		DB.Model(&Message{}).Where(
			"source_user_id = ? AND target_user_id IN ?",
			currentUser.ID, partnerIDs,
		).Order("created_at desc").Find(&newestReplies)
		for _, msg := range newestReplies {
			if _, ok := lastReplyToPartnerIDs[msg.TargetUserID]; ok {
				continue
			}
			lastReplyToPartnerIDs[msg.TargetUserID] = msg
		}
	}

	// Mix the HaveReplied booleans in.
	for _, msg := range m {
		if latest, ok := lastReplyToPartnerIDs[msg.SourceUserID]; ok {
			if latest.CreatedAt.After(msg.CreatedAt) {
				msg.HaveReplied = true
			}
		}
	}
}

// GetMessageThread returns paginated message history between two people.
func GetMessageThread(sourceUserID, targetUserID uint64, pager *Pagination) ([]*Message, error) {
	var m = []*Message{}

	query := DB.Where(
		"(source_user_id = ? AND target_user_id = ?) OR (source_user_id = ? AND target_user_id = ?)",
		sourceUserID, targetUserID,
		targetUserID, sourceUserID,
	).Order(pager.Sort)

	query.Model(&Message{}).Count(&pager.Total)
	result := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&m)
	return m, result.Error
}

// HasMessageThread returns if a message thread exists between two users (either direction).
// Returns the ID of the thread and a boolean OK that it existed.
func HasMessageThread(a, b *User) (uint64, bool) {
	var pager = &Pagination{
		Page:    1,
		PerPage: 1,
		Sort:    "updated_at desc",
	}
	messages, err := GetMessageThread(a.ID, b.ID, pager)
	if err == nil && len(messages) > 0 {
		return messages[0].ID, true
	}
	return 0, false
}

// HasSentAMessage tells if the source user has sent a DM to the target user.
func HasSentAMessage(sourceUser, targetUser *User) bool {
	var count int64
	DB.Model(&Message{}).Where(
		"source_user_id = ? AND target_user_id = ?",
		sourceUser.ID, targetUser.ID,
	).Count(&count)
	return count > 0
}

// DeleteMessageThread removes all message history between two people.
func DeleteMessageThread(message *Message) error {
	return DB.Where(
		"(source_user_id = ? AND target_user_id = ?) OR (source_user_id = ? AND target_user_id = ?)",
		message.SourceUserID, message.TargetUserID,
		message.TargetUserID, message.SourceUserID,
	).Delete(&Message{}).Error
}

// CountUnreadMessages gets the count of unread messages for a user.
func CountUnreadMessages(user *User) (int64, error) {
	var (
		where = []string{
			"target_user_id = ? AND read = ?",
		}
		placeholders = []interface{}{
			user.ID, false,
		}
	)

	// Blocking user IDs?
	bw, bp := BlockedUserSubquery("source_user_id", user.ID)
	where = append(where, bw)
	placeholders = append(placeholders, bp...)

	// Don't show messages from banned or disabled accounts.
	where = append(where, `
		NOT EXISTS (
			SELECT 1
			FROM users
			WHERE users.id IN (messages.target_user_id, messages.source_user_id)
			AND users.status <> 'active'
		)
	`)

	query := DB.Where(
		strings.Join(where, " AND "),
		placeholders...,
	)

	var count int64
	result := query.Model(&Message{}).Count(&count)
	return count, result.Error
}

// SendMessage from a source to a target user.
func SendMessage(sourceUserID, targetUserID uint64, message string) (*Message, error) {
	m := &Message{
		SourceUserID: sourceUserID,
		TargetUserID: targetUserID,
		Message:      message,
		Read:         false,
	}

	result := DB.Create(m)
	return m, result.Error
}

// IsLikelySpam checks if a DM message is likely to be spam so that the front-end can warn the recipient.
//
// This happens e.g. when the sender asks to switch to Telegram or WhatsApp.
func (m *Message) IsLikelySpam() bool {
	body := strings.ToLower(m.Message)
	for _, re := range config.DirectMessageSpamKeywords {
		if idx := re.FindStringIndex(body); len(idx) > 0 {
			return true
		}
	}
	return false
}

// GetMessageInsights returns detailed insights about Direct Message behavior of a user.
func GetMessageInsights(user *User) (*MessageInsight, error) {
	var result = &MessageInsight{
		Items: []*MessageInsightItem{},
	}

	type record struct {
		ContactUserID uint64
		TotalMessages int64
		HasReplied    bool
		LastMessageAt time.Time
	}

	var (
		userID = user.ID
		rows   []record
		res    = DB.Raw(`
			-- All messages that involve the user as sender or recipient
			WITH messages_with_user AS (
				SELECT *
				FROM messages
				WHERE source_user_id = ?
				OR target_user_id = ?
			),

			-- Group the conversations by user ID pairs
			conversation_pairs AS (
				SELECT
					LEAST(source_user_id, target_user_id) AS user_a,
					GREATEST(source_user_id, target_user_id) AS user_b,
					COUNT(*) AS total_messages
				FROM messages_with_user
				GROUP BY LEAST(source_user_id, target_user_id), GREATEST(source_user_id, target_user_id)
			),

			-- Last message timestamps
			last_message_timestamps AS (
			    SELECT
					LEAST(source_user_id, target_user_id) AS user_a,
					GREATEST(source_user_id, target_user_id) AS user_b,
					MAX(created_at) AS last_message_at
				FROM messages_with_user
				GROUP BY LEAST(source_user_id, target_user_id), GREATEST(source_user_id, target_user_id)
			),

			user_conversations AS (
				SELECT
					CASE
						WHEN source_user_id = ? THEN target_user_id
						ELSE source_user_id
					END AS contact_user_id
				FROM messages_with_user
				GROUP BY 1
			),

			user_sent AS (
				SELECT target_user_id
				FROM messages
				WHERE source_user_id = ?
				GROUP BY target_user_id
			),

			contact_replied AS (
				SELECT source_user_id AS contact_user_id
				FROM messages
				WHERE target_user_id = ?
				GROUP BY source_user_id
			),

			final_output AS (
				SELECT
					uc.contact_user_id,
					cp.total_messages,
					lmt.last_message_at,
					CASE WHEN cr.contact_user_id IS NOT NULL THEN TRUE ELSE FALSE END AS has_replied
				FROM user_conversations uc
				LEFT JOIN conversation_pairs cp
					ON (cp.user_a = LEAST(?, uc.contact_user_id) AND cp.user_b = GREATEST(?, uc.contact_user_id))
				LEFT JOIN last_message_timestamps lmt
					ON lmt.user_a = LEAST(?, uc.contact_user_id)
					AND lmt.user_b = GREATEST(?, uc.contact_user_id)
				LEFT JOIN contact_replied cr
					ON uc.contact_user_id = cr.contact_user_id
				ORDER BY lmt.last_message_at DESC
			)

			SELECT * FROM final_output
			`, userID, userID, userID, userID, userID, userID, userID, userID, userID,
		).Scan(&rows)
	)

	if res.Error != nil {
		return result, res.Error
	}

	// Process the message insights.
	var userIDs = []uint64{}
	for _, row := range rows {
		userIDs = append(userIDs, row.ContactUserID)
	}

	userMap, err := MapUsers(user, userIDs)
	if err != nil {
		return result, err
	}

	for _, row := range rows {
		user := userMap[row.ContactUserID]
		result.Items = append(result.Items, &MessageInsightItem{
			ContactUser:   user,
			ContactUserID: row.ContactUserID,
			MessageCount:  row.TotalMessages,
			HasReplied:    row.HasReplied,
			LastMessageAt: row.LastMessageAt,
		})
	}

	return result, nil
}

type MessageInsight struct {
	Items []*MessageInsightItem
}

type MessageInsightItem struct {
	ContactUser   *User
	ContactUserID uint64
	MessageCount  int64
	HasReplied    bool
	LastMessageAt time.Time
}

// Usernames returns the unique usernames from the message insights.
func (mi *MessageInsight) Usernames() []string {
	var (
		usernames = []string{}
		uniq      = map[string]struct{}{}
	)
	for _, item := range mi.Items {
		if item.ContactUser == nil {
			continue
		} else if _, ok := uniq[item.ContactUser.Username]; ok {
			continue
		}

		usernames = append(usernames, item.ContactUser.Username)
		uniq[item.ContactUser.Username] = struct{}{}
	}
	return usernames
}

// UsernameCSV returns the usernames as a comma separated string for front-end convenience.
func (mi *MessageInsight) UsernameCSV() string {
	return strings.Join(mi.Usernames(), ",")
}

// Save message.
func (m *Message) Save() error {
	result := DB.Save(m)
	return result.Error
}

// Delete a message.
func (m *Message) Delete() error {
	return DB.Delete(m).Error
}
