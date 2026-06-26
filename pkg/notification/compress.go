package notification

import (
	"github.com/cuvou/gosocial/pkg/models"
)

type TableKey struct {
	TableName string
	TableID   uint64
}

type UserKey struct {
	UserID    uint64
	TableName string
}

// Compress a page of notifications to group 'like' notifications together.
//
// For example, if a dozen people like your new photo, instead of having 12 Like
// notifications in a row, roll them all up into one compressed notification that
// reads like "alice, bob, and 10 others liked your photo."
func Compress(input []*Notification, maxLength int) []*Notification {
	var (
		result []*Notification

		// Grouping 'Like' notifications by table/ID and by user.
		groupedByTable = map[TableKey][]*Notification{}
		groupedByUser  = map[UserKey][]*Notification{}
		consumed       = map[uint64]any{} // Track which notifications are already grouped
	)

	// Do a first pass through notifications and group the "Likes" by TableName/ID and UserID.
	for _, row := range input {
		if row.Type != models.NotificationLike {
			continue
		}

		var (
			tableName = row.Models[0].TableName
			tableID   = row.Models[0].TableID
			userID    = row.AboutUser.ID

			tableKey = TableKey{tableName, tableID}
			userKey  = UserKey{userID, tableName}
		)

		groupedByTable[tableKey] = append(groupedByTable[tableKey], row)
		groupedByUser[userKey] = append(groupedByUser[userKey], row)
	}

	// Do a second pass through the original Notifications list and compress/group Likes.
	for _, row := range input {
		if _, ok := consumed[row.IDs[0]]; ok {
			continue
		}

		var (
			tableName = row.Models[0].TableName
			tableID   = row.Models[0].TableID
			userID    = row.AboutUser.ID

			tableKey = TableKey{tableName, tableID}
			userKey  = UserKey{userID, tableName}
		)

		// Compress 'Like' notifications (photos/videos only).
		if row.Type == models.NotificationLike && (tableName == "photos" || tableName == "videos") {

			// Case 1: multiple users liked the same picture.
			group := groupedByTable[tableKey]
			if len(group) > 1 {
				for _, other := range group {
					if row == other {
						continue
					}

					// Group unread and read runs of notifications distinctly.
					if row.Read != other.Read {
						continue
					}

					consumed[other.IDs[0]] = nil
					row.IDs = append(row.IDs, other.IDs...)
					row.Models = append(row.Models, other.Models...)

					if !other.Read {
						row.Read = false
						row.OtherUnread++
					}

					if other.AboutUser != nil {
						row.OtherUsernames = append(row.OtherUsernames, other.AboutUser.Username)
					}
				}
			} else if len(group) == 1 {

				// Case 2: see if one user liked multiple pictures.
				group := groupedByUser[userKey]
				if len(group) > 1 {
					for _, other := range group {
						if row == other {
							continue
						}

						// Group unread and read runs of notifications distinctly.
						if row.Read != other.Read {
							continue
						}

						consumed[other.IDs[0]] = nil
						row.IDs = append(row.IDs, other.IDs...)
						row.Models = append(row.Models, other.Models...)

						if !other.Read {
							row.Read = false
							row.OtherUnread++
						}

						row.OtherCount++
					}
				}
			}
		}

		result = append(result, row)
	}

	if maxLength > 0 && len(result) > maxLength {
		result = result[:maxLength]
	}

	return result
}
