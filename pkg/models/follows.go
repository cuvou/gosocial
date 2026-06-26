package models

import (
	"fmt"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/log"
	"gorm.io/gorm/clause"
)

// Follow table.
type Follow struct {
	SourceUserID uint64 `gorm:"uniqueIndex:ix_follow_user_ids"`
	TargetUserID uint64 `gorm:"uniqueIndex:ix_follow_user_ids"`
	CreatedAt    time.Time
	UpdatedAt    time.Time `gorm:"index"`
}

// AddFollow upserts a follow from the source user to the target.
func AddFollow(sourceUserID, targetUserID uint64) (*Follow, error) {
	var (
		f = &Follow{
			SourceUserID: sourceUserID,
			TargetUserID: targetUserID,
		}
		res = DB.Model(&Follow{}).Clauses(
			clause.OnConflict{
				Columns: []clause.Column{
					{Name: "source_user_id"},
					{Name: "target_user_id"},
				},
				UpdateAll: true,
			},
		).Create(&f)
	)
	return f, res.Error
}

// Unfollow removes a follow from the source user to the target.
func Unfollow(sourceUserID, targetUserID uint64) error {
	return DB.Where(
		"source_user_id = ? AND target_user_id = ?",
		sourceUserID, targetUserID,
	).Delete(&Follow{}).Error
}

// UnfollowMutually breaks a follow in both directions, e.g. for blocking.
func UnfollowMutually(sourceUserID, targetUserID uint64) error {
	return DB.Where(
		"(source_user_id = ? AND target_user_id = ?) OR (target_user_id = ? AND source_user_id = ?)",
		sourceUserID, targetUserID, sourceUserID, targetUserID,
	).Delete(&Follow{}).Error
}

// IsFollowing returns whether the current user follows the target.
func IsFollowing(sourceUserID, targetUserID uint64) bool {
	var (
		found bool
		res   = DB.Raw(
			"SELECT true FROM follows WHERE source_user_id = ? AND target_user_id = ?",
			sourceUserID, targetUserID,
		).Scan(&found)
	)
	if res.Error != nil {
		log.Error("IsFollowing(%d, %d): %s", sourceUserID, targetUserID, res.Error)
	}
	return found
}

// PaginateFollowers looks through a user's follow lists.
func PaginateFollowers(currentUser *User, isFollowing bool, excludeFriends bool, pager *Pagination) ([]*User, error) {
	var (
		fs              = []*Follow{}
		wheres          = []string{}
		placeholders    = []any{}
		otherUserColumn string
	)

	// Which list?
	if !isFollowing {
		// My followers, where the target_user_id is the currentUser.
		wheres = append(wheres, "follows.target_user_id = ?")
		placeholders = append(placeholders, currentUser.ID)
		otherUserColumn = "follows.source_user_id"
	} else {
		// My following list, where the target_user_id are the users I follow.
		wheres = append(wheres, "follows.source_user_id = ?")
		placeholders = append(placeholders, currentUser.ID)
		otherUserColumn = "follows.target_user_id"
	}

	// Excluding friends?
	if excludeFriends {
		wheres = append(wheres, fmt.Sprintf(`
			NOT EXISTS (
				SELECT 1
				FROM friends
				WHERE friends.source_user_id = ?
				AND friends.target_user_id = %s
			)
		`, otherUserColumn))
		placeholders = append(placeholders, currentUser.ID)
	}

	// Only active users.
	wheres = append(wheres, "users.status = ?")
	placeholders = append(placeholders, UserStatusActive)

	// Exclude blocked users.
	bw, bp := BlockedUserSubquery("users.id", currentUser.ID)
	wheres = append(wheres, bw)
	placeholders = append(placeholders, bp...)

	// Paginate the Follows table.
	query := DB.Model(&Follow{}).Joins(
		fmt.Sprintf("JOIN users ON (%s = users.id)", otherUserColumn),
	).Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Order(pager.Sort)

	query.Count(&pager.Total)
	res := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&fs)
	if res.Error != nil {
		return nil, res.Error
	}

	// Map the user IDs.
	var userIDs []uint64
	for _, row := range fs {
		if !isFollowing {
			userIDs = append(userIDs, row.SourceUserID)
		} else {
			userIDs = append(userIDs, row.TargetUserID)
		}
	}

	return GetUsers(currentUser, userIDs)
}

// CountFollows returns the counts of followers and following for the current user.
func CountFollows(currentUser *User) (followers, following int64, err error) {
	type record struct {
		MetricName  string
		MetricValue int64
	}
	var (
		records []record
		res     = DB.Raw(
			`
				WITH subquery_followers AS (
					SELECT COUNT(*) AS c
					FROM follows
					JOIN users ON (users.id = follows.source_user_id)
					WHERE target_user_id = ?
					AND users.status = 'active'
				),

				subquery_following AS (
					SELECT COUNT(*) AS c
					FROM follows
					JOIN users ON (users.id = follows.target_user_id)
					WHERE source_user_id = ?
					AND users.status = 'active'
				)

				SELECT
					'Followers' AS metric_name,
					c AS metric_value
				FROM subquery_followers

				UNION ALL

				SELECT
					'Following' AS metric_name,
					c AS metric_value
				FROM subquery_following
			`,
			currentUser.ID, currentUser.ID,
		).Scan(&records)
	)
	if res.Error != nil {
		return 0, 0, res.Error
	}

	for _, row := range records {
		switch row.MetricName {
		case "Followers":
			followers = row.MetricValue
		case "Following":
			following = row.MetricValue
		default:
			return 0, 0, fmt.Errorf("unexpected metric from SQL query: %s", row.MetricName)
		}
	}

	return followers, following, nil
}

// FollowerIDs returns all user IDs that follow the user.
func FollowerIDs(userId uint64) []uint64 {
	var (
		fs      = []*Follow{}
		userIDs = []uint64{}
	)
	DB.Where("target_user_id = ?", userId).Find(&fs)
	for _, row := range fs {
		userIDs = append(userIDs, row.SourceUserID)
	}
	return userIDs
}

// FollowerIDsAreExplicit returns follower user IDs who have opted-in for Explicit content,
// e.g. to notify only them when you uploaded a new Explicit photo so that non-explicit
// users don't need to see that notification.
func FollowerIDsAreExplicit(userId uint64) []uint64 {
	var (
		userIDs = []uint64{}
	)

	err := DB.Table(
		"follows",
	).Joins(
		"JOIN users ON (users.id = follows.source_user_id)",
	).Select(
		"follows.source_user_id AS follower_id",
	).Where(
		"follows.target_user_id = ? AND users.explicit IS TRUE",
		userId,
	).Scan(&userIDs)

	if err.Error != nil {
		log.Error("SQL error collecting explicit FollowerIDs for %d: %s", userId, err.Error)
	}

	return userIDs
}

// FilterFollowerIDs can filter down a listing of user IDs and return only the ones who are your followers.
func FilterFollowerIDs(userIDs []uint64, followerIDs []uint64) []uint64 {
	var (
		seen     = map[uint64]any{}
		filtered = []uint64{}
	)

	// Map the friend IDs out.
	for _, friendID := range followerIDs {
		seen[friendID] = nil
	}

	// Filter the userIDs.
	for _, userID := range userIDs {
		if _, ok := seen[userID]; ok {
			filtered = append(filtered, userID)
		}
	}

	return filtered
}

// MapFollows maps a set of user IDs to their follow status regarding the current user.
func MapFollows(currentUser *User, userIDs []uint64) FollowMap {
	var (
		result = FollowMap{}
		fs     = []*Follow{}
		res    = DB.Model(&Follow{}).Where(
			`
				(source_user_id = ? AND target_user_id IN ?)
				OR (target_user_id = ? AND source_user_id IN ?)
			`,
			currentUser.ID, userIDs,
			currentUser.ID, userIDs,
		).Find(&fs)
	)
	if res.Error != nil {
		log.Error("MapFollows(%s): %s", currentUser.Username, res.Error)
		return result
	}

	// Map the results in.
	for _, row := range fs {

		// Follow direction and the relevant (other) user ID.
		var (
			otherUserID uint64
			isFollowing bool
		)
		if row.SourceUserID == currentUser.ID {
			otherUserID = row.TargetUserID
			isFollowing = true
		} else {
			otherUserID = row.SourceUserID
		}

		// Already have one direction?
		_, exists := result[otherUserID]

		if isFollowing {
			if exists {
				result[otherUserID] = "mutual"
			} else {
				result[otherUserID] = "following"
			}
		} else {
			if exists {
				result[otherUserID] = "mutual"
			} else {
				result[otherUserID] = "follower"
			}
		}
	}

	return result
}

// FollowMap maps follower status to a set of user IDs.
type FollowMap map[uint64]string

// IsMutual returns if the user ID is a mutual follower.
func (fm FollowMap) IsMutual(userID uint64) bool {
	return fm[userID] == "mutual"
}

// IsFollower returns if the user ID is a follower (not mutual).
func (fm FollowMap) IsFollower(userID uint64) bool {
	return fm[userID] == "follower"
}

// IsFollowing returns if the user ID is following (not mutual).
func (fm FollowMap) IsFollowing(userID uint64) bool {
	return fm[userID] == "following"
}
