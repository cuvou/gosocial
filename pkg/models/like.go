package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/log"
)

// Like table.
type Like struct {
	ID        uint64    `gorm:"primaryKey"`
	UserID    uint64    `gorm:"index;index:idx_likes_user_composite"` // who it belongs to
	TableName string    `gorm:"index:idx_likes_composite;index:idx_likes_user_composite"`
	TableID   uint64    `gorm:"index:idx_likes_composite;index:idx_likes_user_composite"`
	CreatedAt time.Time `gorm:"index"`
	UpdatedAt time.Time
}

// LikeableTables are the set of table names that allow likes (used by the JSON API).
var LikeableTables = map[string]interface{}{
	"photos":   nil,
	"users":    nil,
	"comments": nil,
	"blogs":    nil,
}

// AddLike to something.
func AddLike(user *User, tableName string, tableID uint64) error {
	// Already has a like?
	var like = &Like{}
	exist := DB.Model(like).Where(
		"user_id = ? AND table_name = ? AND table_id = ?",
		user.ID, tableName, tableID,
	).First(&like)
	if exist.Error == nil {
		return nil
	}

	// Create it.
	like = &Like{
		UserID:    user.ID,
		TableName: tableName,
		TableID:   tableID,
	}
	return DB.Create(like).Error
}

// Unlike something.
func Unlike(user *User, tableName string, tableID uint64) error {
	result := DB.Where(
		"user_id = ? AND table_name = ? AND table_id = ?",
		user.ID, tableName, tableID,
	).Delete(&Like{})
	return result.Error
}

// CountLikes on something.
func CountLikes(tableName string, tableID uint64) int64 {
	var count int64
	DB.Model(&Like{}).Where(
		"table_name = ? AND table_id = ?",
		tableName, tableID,
	).Count(&count)
	return count
}

// CountLikesGiven by a user.
func CountLikesGiven(user *User) int64 {
	var count int64
	DB.Model(&Like{}).Where(
		"user_id = ?",
		user.ID,
	).Count(&count)
	return count
}

// CountLikesReceived by a user.
func CountLikesReceived(user *User) int64 {
	var count int64

	// Do a UNION query as it's more efficient than joining Likes to all the other tables.
	result := DB.Raw(`
		SELECT sum(c) FROM (
			SELECT count(*) AS c
			FROM likes
			WHERE likes.table_name = 'users' AND likes.table_id = ?

			UNION

			SELECT count(*) AS c
			FROM likes
			JOIN photos ON (likes.table_name='photos' AND likes.table_id=photos.id)
			WHERE photos.user_id = ?

			UNION

			SELECT count(*) AS c
			FROM likes
			JOIN comments ON (likes.table_name = 'comments' AND likes.table_id=comments.id)
			WHERE comments.user_id = ?
		) AS c
	`, user.ID, user.ID, user.ID).Scan(&count)

	if result.Error != nil {
		log.Error("CountLikesReceived: %s", result.Error)
	}

	return count
}

// WhoLikes something. Returns the first couple users and a count of the remainder.
func WhoLikes(currentUser *User, tableName string, tableID uint64) ([]*User, int64, error) {
	var (
		userIDs      = []uint64{}
		likes        = []*Like{}
		total        = CountLikes(tableName, tableID)
		remainder    = total
		wheres       = []string{}
		placeholders = []interface{}{}
	)

	wheres = append(wheres, "table_name = ? AND table_id = ?")
	placeholders = append(placeholders, tableName, tableID)

	// Blocking user IDs?
	bw, bp := BlockedUserSubquery("user_id", currentUser.ID)
	wheres = append(wheres, bw)
	placeholders = append(placeholders, bp...)

	res := DB.Model(&Like{}).Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Order("created_at DESC").Limit(2).Scan(&likes)

	if res.Error != nil {
		return nil, 0, res.Error
	}

	// Subtract the (up to two) likes from the total.
	remainder -= int64(len(likes))

	// Collect the user IDs to look up.
	for _, row := range likes {
		userIDs = append(userIDs, row.UserID)
	}

	// Look up the users and return the remainder.
	users, err := GetUsers(currentUser, userIDs)
	if err != nil {
		return nil, 0, err
	}

	return users, remainder, nil
}

// PaginateLikes returns a paged view of users who've liked something.
//
// friendsOnly can be a string of "true" or "false" to filter for/for not friends, otherwise returns all likes.
func PaginateLikes(currentUser *User, tableName string, tableID uint64, friendsOnly string, pager *Pagination) ([]*User, error) {
	var (
		l            = []*Like{}
		userIDs      = []uint64{}
		wheres       = []string{}
		placeholders = []any{}
	)

	wheres = append(wheres, "table_name = ? AND table_id = ?")
	placeholders = append(placeholders, tableName, tableID)

	// Blocking user IDs?
	bw, bp := BlockedUserSubquery("user_id", currentUser.ID)
	wheres = append(wheres, bw)
	placeholders = append(placeholders, bp...)

	// Active accounts only.
	wheres = append(wheres, `
		EXISTS (
			SELECT 1
			FROM users
			WHERE users.id = likes.user_id
			AND users.status = ?
		)
	`)
	placeholders = append(placeholders, UserStatusActive)

	// Friends only?
	if friendsOnly == "true" || friendsOnly == "false" {
		not := ""
		if friendsOnly == "false" {
			not = "NOT "
		}
		wheres = append(wheres, fmt.Sprintf(`
			%sEXISTS (
				SELECT 1
				FROM friends
				WHERE source_user_id = ?
				AND target_user_id = likes.user_id
				AND approved IS TRUE
			)
		`, not))
		placeholders = append(placeholders, currentUser.ID)
	}

	query := DB.Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Order(
		pager.Sort,
	)

	// Get the total count.
	query.Model(&Like{}).Count(&pager.Total)

	// Get the page of likes.
	result := query.Offset(
		pager.GetOffset(),
	).Limit(pager.PerPage).Find(&l)
	if result.Error != nil {
		return nil, result.Error
	}

	// Map the user IDs in.
	for _, like := range l {
		userIDs = append(userIDs, like.UserID)
	}
	return GetUsers(currentUser, userIDs)
}

// LikedIDs filters a set of table IDs to ones the user likes.
func LikedIDs(user *User, tableName string, tableIDs []uint64) ([]uint64, error) {
	var result = []uint64{}
	if r := DB.Table(
		"likes",
	).Select(
		"table_id",
	).Where(
		"user_id = ? AND table_name = ? AND table_id IN ?",
		user.ID, tableName, tableIDs,
	).Scan(&result); r.Error != nil {
		return result, r.Error
	}

	return result, nil
}

// LikeMap maps table IDs to Likes metadata.
type LikeMap map[uint64]*LikeStats

// Get like stats from the map.
func (lm LikeMap) Get(id uint64) *LikeStats {
	if stats, ok := lm[id]; ok {
		return stats
	}
	return &LikeStats{}
}

// LikeStats holds mapped statistics about liked objects.
type LikeStats struct {
	Count     int64 // how many total
	UserLikes bool  // current user likes it
}

// MapLikes over a set of table IDs.
func MapLikes(user *User, tableName string, tableIDs []uint64) LikeMap {
	var result = LikeMap{}

	// Initialize the result set.
	for _, id := range tableIDs {
		result[id] = &LikeStats{}
	}

	// Hold the result of the grouped count query.
	type group struct {
		ID    uint64
		Likes int64
	}
	var groups = []group{}

	// Map the counts of likes to each of these IDs.
	if res := DB.Table(
		"likes",
	).Select(
		"table_id AS id, count(id) AS likes",
	).Where(
		"table_name = ? AND table_id IN ?",
		tableName, tableIDs,
	).Group("table_id").Scan(&groups); res.Error != nil {
		log.Error("MapLikes: count query: %s", res.Error)
	}

	// Map the counts back in.
	for _, row := range groups {
		if stats, ok := result[row.ID]; ok {
			stats.Count = row.Likes
		}
	}

	// Does the CURRENT USER like any of these IDs?
	if likedIDs, err := LikedIDs(user, tableName, tableIDs); err == nil {
		for _, id := range likedIDs {
			if stats, ok := result[id]; ok {
				stats.UserLikes = true
			}
		}
	}

	return result
}
