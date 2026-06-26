package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/log"
)

// Block table.
type Block struct {
	ID           uint64 `gorm:"primaryKey"`
	SourceUserID uint64 `gorm:"index"`
	TargetUserID uint64 `gorm:"index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// AddBlock is sourceUserId adding targetUserId to their block list.
func AddBlock(sourceUserID, targetUserID uint64) error {
	// Unfriend in the process (if the target has not ignored our request).
	if !HasIgnoredFriendRequest(sourceUserID, targetUserID) {
		RemoveFriend(sourceUserID, targetUserID)
	}

	// Break follows.
	if err := UnfollowMutually(sourceUserID, targetUserID); err != nil {
		log.Error("AddBlock(%d, %d): unfollow mutually: %s", sourceUserID, targetUserID, err)
	}

	// Did we already block this user?
	var b *Block
	forward := DB.Where(
		"source_user_id = ? AND target_user_id = ?",
		sourceUserID, targetUserID,
	).First(&b).Error

	// Update existing.
	if forward == nil {
		return nil
	}

	// Create the block.
	b = &Block{
		SourceUserID: sourceUserID,
		TargetUserID: targetUserID,
	}
	return DB.Create(b).Error
}

// IsBlocking quickly sees if either user blocks the other.
func IsBlocking(sourceUserID, targetUserID uint64) bool {
	b := &Block{}
	result := DB.Where(
		"(source_user_id = ? AND target_user_id = ?) OR "+
			"(target_user_id = ? AND source_user_id = ?)",
		sourceUserID, targetUserID,
		sourceUserID, targetUserID,
	).First(&b)
	return result.Error == nil
}

// IsBlocked quickly checks if sourceUserID currently blocks targetUserID.
func IsBlocked(sourceUserID, targetUserID uint64) bool {
	b := &Block{}
	result := DB.Where(
		"source_user_id = ? AND target_user_id = ?",
		sourceUserID, targetUserID,
	).First(&b)
	return result.Error == nil
}

// BlockDirections checks both sides of a block list between the source and target user.
//
// Forward list means source user has blocked the target. Reverse is the target blocking the source.
func BlockDirections(sourceUserID, targetUserID uint64) (forward, reverse bool) {
	var (
		bs     = []*Block{}
		result = DB.Model(&Block{}).Where(
			"(source_user_id = ? AND target_user_id = ?) OR "+
				"(target_user_id = ? AND source_user_id = ?)",
			sourceUserID, targetUserID,
			sourceUserID, targetUserID,
		).Scan(&bs)
	)
	if result.Error != nil {
		log.Error("BlockDirections(%d, %d): %s", sourceUserID, targetUserID, result.Error)
		return
	}

	for _, row := range bs {
		if row.SourceUserID == sourceUserID {
			forward = true
		} else if row.TargetUserID == sourceUserID {
			reverse = true
		}
	}

	return
}

// PaginateBlockList views a user's blocklist, optionally searching for a name or username.
func PaginateBlockList(user *User, search string, pager *Pagination) ([]*User, error) {
	// We paginate over the Block table.
	var (
		bs           = []*Block{}
		userIDs      = []uint64{}
		query        = DB.Model(&Block{})
		wheres       = []string{}
		placeholders = []interface{}{}
	)

	wheres = append(wheres, "blocks.source_user_id = ?")
	placeholders = append(placeholders, user.ID)

	if search != "" {
		like := "%" + search + "%"
		query = DB.Joins(
			"JOIN users ON (users.id = blocks.target_user_id)",
		)

		wheres = append(wheres, "(users.username ILIKE ? OR users.name ILIKE ?)")
		placeholders = append(placeholders, like, like)
	}

	query = query.Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	)

	query = query.Order(pager.Sort)
	query.Model(&Block{}).Count(&pager.Total)
	result := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&bs)
	if result.Error != nil {
		return nil, result.Error
	}

	// Now of these friends get their User objects.
	for _, b := range bs {
		userIDs = append(userIDs, b.TargetUserID)
	}

	return GetUsers(user, userIDs)
}

/*
BlockedUserSubquery returns the WHERE clause and placeholders to exclude blocked users
from other queries around the site.

The userColumn is the column in your actual query that you want to exclude blocking user
IDs from. For example it will usually be 'user_id' for a table such as Photos, or maybe
a 'source_user_id' when checking the user's received Messages.

The userID is usually the current user's ID. If a block exists having this userID as either
the source or target, this will add a `WHERE userColumn NOT IN` subquery to prevent the
row from being selected.
*/
func BlockedUserSubquery(userColumn string, userID uint64) (where string, placeholders []any) {
	where = fmt.Sprintf(`(%s NOT IN (
		SELECT source_user_id
		FROM blocks
		WHERE target_user_id = ?
	) AND %s NOT IN (
		SELECT target_user_id
		FROM blocks
		WHERE source_user_id = ?
	))`, userColumn, userColumn)
	placeholders = append(placeholders, userID, userID)
	return
}

// BlockedUserIDs returns all user IDs blocked by the user (bidirectional, source or target user).
func BlockedUserIDs(user *User) []uint64 {
	// Have we looked this up already on this request?
	if user.cacheBlockedUserIDs != nil {
		return user.cacheBlockedUserIDs
	}

	var (
		bs      = []*Block{}
		userIDs = []uint64{}
	)
	DB.Where("source_user_id = ? OR target_user_id = ?", user.ID, user.ID).Find(&bs)
	for _, row := range bs {
		for _, uid := range []uint64{row.TargetUserID, row.SourceUserID} {
			if uid != user.ID {
				userIDs = append(userIDs, uid)
			}
		}
	}

	// Cache the result in the User so we don't query it again.
	user.cacheBlockedUserIDs = userIDs

	return userIDs
}

// MapBlockedUserIDs returns BlockedUserIDs as a lookup hashmap (not for front-end templates currently).
func MapBlockedUserIDs(user *User) map[uint64]interface{} {
	var (
		result     = map[uint64]interface{}{}
		blockedIDs = BlockedUserIDs(user)
	)
	for _, uid := range blockedIDs {
		result[uid] = nil
	}
	return result
}

// FilterBlockingUserIDs narrows down a set of User IDs to remove ones that block (or are blocked by) the current user.
func FilterBlockingUserIDs(currentUser *User, userIDs []uint64) []uint64 {
	var (
		// Get the IDs to exclude.
		blockedIDs = MapBlockedUserIDs(currentUser)

		// Filter the result.
		result = []uint64{}
	)
	for _, uid := range userIDs {
		if _, ok := blockedIDs[uid]; ok {
			continue
		}
		result = append(result, uid)
	}
	return result
}

// BlockedUserIDsByUser returns all user IDs blocked by the user (one directional only)
func BlockedUserIDsByUser(userId uint64) []uint64 {
	var (
		bs      = []*Block{}
		userIDs = []uint64{}
	)
	DB.Where("source_user_id = ?", userId).Find(&bs)
	for _, row := range bs {
		for _, uid := range []uint64{row.TargetUserID, row.SourceUserID} {
			if uid != userId {
				userIDs = append(userIDs, uid)
			}
		}
	}
	return userIDs
}

// GetAllBlockedUserIDs returns the forward and reverse lists of blocked user IDs for the user.
func GetAllBlockedUserIDs(user *User) (forward, reverse []uint64) {
	var (
		bs = []*Block{}
	)
	DB.Where("source_user_id = ? OR target_user_id = ?", user.ID, user.ID).Find(&bs)
	for _, row := range bs {
		if row.SourceUserID == user.ID {
			forward = append(forward, row.TargetUserID)
		} else if row.TargetUserID == user.ID {
			reverse = append(reverse, row.SourceUserID)
		}
	}
	return forward, reverse
}

// BulkRestoreBlockedUserIDs inserts many blocked user IDs in one query.
//
// Returns the count of blocks added.
func BulkRestoreBlockedUserIDs(user *User, forward, reverse []uint64) (int, error) {
	var bs = []*Block{}

	// Forward list.
	for _, uid := range forward {
		bs = append(bs, &Block{
			SourceUserID: user.ID,
			TargetUserID: uid,
		})
	}

	// Reverse list.
	for _, uid := range reverse {
		bs = append(bs, &Block{
			SourceUserID: uid,
			TargetUserID: user.ID,
		})
	}

	// Anything to do?
	if len(bs) == 0 {
		return 0, nil
	}

	// Batch create.
	res := DB.Create(bs)
	return len(bs), res.Error
}

// BlockedUsernames returns all usernames blocked by (or blocking) the user.
func BlockedUsernames(user *User) []string {
	var (
		userIDs   = BlockedUserIDs(user)
		usernames = []string{}
	)

	if len(userIDs) == 0 {
		return usernames
	}

	if res := DB.Table(
		"users",
	).Select(
		"username",
	).Where(
		"id IN ?", userIDs,
	).Scan(&usernames); res.Error != nil {
		log.Error("BlockedUsernames(%s): %s", user.Username, res.Error)
	}

	return usernames
}

// GetBlocklistInsights returns detailed block lists (both directions) about a user, for admin insight.
func GetBlocklistInsights(user *User) (*BlocklistInsight, error) {
	// Collect ALL user IDs (both directions) of this user's blocklist.
	var (
		bs        = []*Block{}
		forward   = []*Block{} // Users they block
		reverse   = []*Block{} // Users who block the target
		userIDs   = []uint64{user.ID}
		usernames = map[uint64]string{}
		admins    = map[uint64]bool{}
	)

	// Get the complete blocklist and bucket them into forward and reverse.
	DB.Where("source_user_id = ? OR target_user_id = ?", user.ID, user.ID).Order("created_at desc").Find(&bs)
	for _, row := range bs {
		if row.SourceUserID == user.ID {
			forward = append(forward, row)
			userIDs = append(userIDs, row.TargetUserID)
		} else {
			reverse = append(reverse, row)
			userIDs = append(userIDs, row.SourceUserID)
		}
	}

	// Map all the user IDs to user names.
	if len(userIDs) > 0 {
		type scanItem struct {
			ID       uint64
			Username string
			IsAdmin  bool
		}
		var scan = []scanItem{}
		if res := DB.Table(
			"users",
		).Select(
			"id",
			"username",
			"is_admin",
		).Where(
			"id IN ?", userIDs,
		).Scan(&scan); res.Error != nil {
			return nil, fmt.Errorf("GetBlocklistInsights(%s): mapping user IDs to names: %s", user.Username, res.Error)
		}

		for _, row := range scan {
			usernames[row.ID] = row.Username
			admins[row.ID] = row.IsAdmin
		}
	}

	// Assemble the final result.
	var result = &BlocklistInsight{
		Blocks:    []BlocklistInsightUser{},
		BlockedBy: []BlocklistInsightUser{},
	}
	for _, row := range forward {
		if username, ok := usernames[row.TargetUserID]; ok {
			result.Blocks = append(result.Blocks, BlocklistInsightUser{
				Username: username,
				IsAdmin:  admins[row.TargetUserID],
				Date:     row.CreatedAt,
			})
		}
	}
	for _, row := range reverse {
		if username, ok := usernames[row.SourceUserID]; ok {
			result.BlockedBy = append(result.BlockedBy, BlocklistInsightUser{
				Username: username,
				IsAdmin:  admins[row.SourceUserID],
				Date:     row.CreatedAt,
			})
		}
	}

	return result, nil
}

type BlocklistInsight struct {
	Blocks    []BlocklistInsightUser
	BlockedBy []BlocklistInsightUser
}

type BlocklistInsightUser struct {
	Username string
	IsAdmin  bool
	Date     time.Time
}

// UnblockUser removes targetUserID from your blocklist.
func UnblockUser(sourceUserID, targetUserID uint64) error {
	result := DB.Where(
		"source_user_id = ? AND target_user_id = ?",
		sourceUserID, targetUserID,
	).Delete(&Block{})
	return result.Error
}

// Save photo.
func (b *Block) Save() error {
	result := DB.Save(b)
	return result.Error
}
