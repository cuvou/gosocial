package models

import (
	"errors"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"gorm.io/gorm"
)

// ForumMembership table.
//
// Unique key constraint pairs user_id and forum_id.
type ForumMembership struct {
	ID          uint64 `gorm:"primaryKey"`
	UserID      uint64 `gorm:"uniqueIndex:idx_forum_membership"`
	User        User   `gorm:"foreignKey:user_id"`
	ForumID     uint64 `gorm:"uniqueIndex:idx_forum_membership"`
	Forum       Forum  `gorm:"foreignKey:forum_id"`
	Approved    bool   `gorm:"index"`
	IsModerator bool   `gorm:"index"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// Preload related tables for the forum (classmethod).
func (f *ForumMembership) Preload() *gorm.DB {
	return DB.Preload("User").Preload("Forum")
}

// CreateForumMembership subscribes the user to a forum.
func CreateForumMembership(userID, forumID uint64) (*ForumMembership, error) {
	var (
		f = &ForumMembership{
			UserID:   userID,
			ForumID:  forumID,
			Approved: true,
		}
		result = DB.Create(f)
	)
	return f, result.Error
}

// GetForumMembership looks up a forum membership, returning an error if one is not found.
func GetForumMembership(userID, forumID uint64) (*ForumMembership, error) {
	var (
		f      = &ForumMembership{}
		result = f.Preload().Where(
			"user_id = ? AND forum_id = ?",
			userID, forumID,
		).First(&f)
	)
	return f, result.Error
}

// AddModerator appoints a moderator to the forum, returning that user's ForumMembership.
//
// If the target is not following the forum, a ForumMembership is created, marked as a moderator and returned.
func (f *Forum) AddModerator(user *User) (*ForumMembership, error) {
	var fm *ForumMembership
	if found, err := GetForumMembership(user.ID, f.ID); err != nil {
		fm = &ForumMembership{
			User:     *user,
			Forum:    *f,
			Approved: true,
		}
	} else {
		fm = found
	}

	// They are already a moderator?
	if fm.IsModerator {
		return fm, errors.New("they are already a moderator of this forum")
	}

	fm.IsModerator = true
	err := fm.Save()
	return fm, err
}

// CanBeSeenBy checks whether the user can see a private forum.
//
// Admins, owners, moderators and approved followers can see it.
//
// Note: this may invoke a DB query to check for moderator.
func (f *Forum) CanBeSeenBy(user *User) bool {
	if !f.Private || user.HasAdminScope(config.ScopeAdminBase) || user.ID == f.OwnerID {
		return true
	}

	if fm, err := GetForumMembership(user.ID, f.ID); err == nil {
		return fm.Approved || fm.IsModerator
	}

	return false
}

// CanBeModeratedBy checks whether the user can moderate this forum.
//
// Admins, owners and moderators can do so.
//
// Note: this may invoke a DB query to check for moderator.
func (f *Forum) CanBeModeratedBy(user *User) bool {
	if user.HasAdminScope(config.ScopeForumModerator) || f.OwnerID == user.ID {
		return true
	}

	if fm, err := GetForumMembership(user.ID, f.ID); err == nil {
		return fm.IsModerator
	}

	return false
}

// RemoveModerator will unset a user's moderator flag on this forum.
func (f *Forum) RemoveModerator(user *User) (*ForumMembership, error) {
	fm, err := GetForumMembership(user.ID, f.ID)
	if err != nil {
		return nil, err
	}

	fm.IsModerator = false
	err = fm.Save()
	return fm, err
}

// GetModerators loads all of the moderators of a forum, ordered alphabetically by username.
func (f *Forum) GetModerators() ([]*User, error) {
	// Find all forum memberships that moderate us.
	var (
		fm     = []*ForumMembership{}
		result = (&ForumMembership{}).Preload().Where(
			"forum_id = ? AND is_moderator IS TRUE",
			f.ID,
		).Find(&fm)
	)
	if result.Error != nil {
		log.Error("Forum(%d).GetModerators(): %s", f.ID, result.Error)
		return nil, nil
	}

	// Load these users.
	var userIDs = []uint64{}
	for _, row := range fm {
		userIDs = append(userIDs, row.UserID)
	}

	return GetUsersAlphabetically(userIDs)
}

// IsForumSubscribed checks if the current user subscribes to this forum.
func IsForumSubscribed(userID, forumID uint64) bool {
	f, _ := GetForumMembership(userID, forumID)
	return f.UserID == userID
}

// HasForumSubscriptions returns if the current user has at least one forum membership.
func (u *User) HasForumSubscriptions() bool {
	var count int64
	DB.Model(&ForumMembership{}).Where(
		"user_id = ?",
		u.ID,
	).Count(&count)
	return count > 0
}

// CountForumMemberships counts how many subscribers a forum has.
func CountForumMemberships(forum *Forum) int64 {
	var count int64
	DB.Model(&ForumMembership{}).Where(
		"forum_id = ?",
		forum.ID,
	).Count(&count)
	return count
}

// Save a forum membership.
func (f *ForumMembership) Save() error {
	return DB.Save(f).Error
}

// Delete a forum membership.
func (f *ForumMembership) Delete() error {
	return DB.Delete(f).Error
}

// PaginateForumMemberships paginates over a user's ForumMemberships.
func PaginateForumMemberships(user *User, pager *Pagination) ([]*ForumMembership, error) {
	var (
		fs           = []*ForumMembership{}
		query        = (&ForumMembership{}).Preload()
		wheres       = []string{}
		placeholders = []interface{}{}
	)

	query = query.Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Order(pager.Sort)

	query.Model(&ForumMembership{}).Count(&pager.Total)
	result := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&fs)
	return fs, result.Error
}

// ForumMembershipMap maps table IDs to Likes metadata.
type ForumMembershipMap map[uint64]bool

// Get like stats from the map.
func (fm ForumMembershipMap) Get(id uint64) bool {
	return fm[id]
}

// MapForumMemberships looks up a user's memberships in bulk.
func MapForumMemberships(user *User, forums []*Forum) ForumMembershipMap {
	var (
		result   = ForumMembershipMap{}
		forumIDs = []uint64{}
	)

	// Initialize the result set.
	for _, forum := range forums {
		result[forum.ID] = false
		forumIDs = append(forumIDs, forum.ID)
	}

	// Map the forum IDs the user subscribes to.
	var followIDs = []uint64{}
	if res := DB.Model(&ForumMembership{}).Select(
		"forum_id",
	).Where(
		"user_id = ? AND forum_id IN ?",
		user.ID, forumIDs,
	).Scan(&followIDs); res.Error != nil {
		log.Error("MapForumMemberships: %s", res.Error)
	}

	for _, forumID := range followIDs {
		result[forumID] = true
	}

	return result
}

// ForumFollowerMap maps table IDs to counts of memberships.
type ForumFollowerMap map[uint64]int64

// Get like stats from the map.
func (fm ForumFollowerMap) Get(id uint64) int64 {
	return fm[id]
}

// MapForumFollowers maps out the count of followers for a set of forums.
func MapForumFollowers(forums []*Forum) ForumFollowerMap {
	var (
		result   = ForumFollowerMap{}
		forumIDs = []uint64{}
	)

	// Initialize the result set.
	for _, forum := range forums {
		forumIDs = append(forumIDs, forum.ID)
	}

	// Hold the result of the grouped count query.
	type group struct {
		ID        uint64
		Followers int64
	}
	var groups = []group{}

	// Map the counts of likes to each of these IDs.
	if res := DB.Model(
		&ForumMembership{},
	).Select(
		"forum_id AS id, count(id) AS followers",
	).Where(
		"forum_id IN ?",
		forumIDs,
	).Group("forum_id").Scan(&groups); res.Error != nil {
		log.Error("MapLikes: count query: %s", res.Error)
	}

	for _, row := range groups {
		result[row.ID] = row.Followers
	}

	return result
}
