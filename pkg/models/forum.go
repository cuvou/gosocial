package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"gorm.io/gorm"
)

// Forum table.
type Forum struct {
	ID           uint64 `gorm:"primaryKey"`
	OwnerID      uint64 `gorm:"index"`
	Owner        User   `gorm:"foreignKey:owner_id"`
	Category     string `gorm:"index"`
	Fragment     string `gorm:"uniqueIndex"`
	Title        string
	Description  string
	Explicit     bool `gorm:"index"`
	Privileged   bool
	PermitPhotos bool
	Private      bool `gorm:"index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// Preload related tables for the forum (classmethod).
func (f *Forum) Preload() *gorm.DB {
	return DB.Preload("Owner").Preload("Owner.ProfilePhoto")
}

// GetForum by ID.
func GetForum(id uint64) (*Forum, error) {
	forum := &Forum{}
	result := forum.Preload().First(&forum, id)
	return forum, result.Error
}

// GetForums queries a set of thread IDs and returns them mapped.
func GetForums(IDs []uint64) (map[uint64]*Forum, error) {
	var (
		mt = map[uint64]*Forum{}
		fs = []*Forum{}
	)

	result := (&Forum{}).Preload().Where("id IN ?", IDs).Find(&fs)
	for _, row := range fs {
		mt[row.ID] = row
	}

	return mt, result.Error
}

// ForumByFragment looks up a forum by its URL fragment.
func ForumByFragment(fragment string) (*Forum, error) {
	if fragment == "" {
		return nil, errors.New("the URL fragment is required")
	}

	var (
		f      = &Forum{}
		result = f.Preload().Where(
			"fragment = ?",
			fragment,
		).First(&f)
	)

	return f, result.Error
}

// CanEdit checks if the user has edit rights over this forum.
//
// That is, they are its Owner or they are an admin with Manage Forums permission.
func (f *Forum) CanEdit(user *User) bool {
	return user.HasAdminScope(config.ScopeForumAdmin) || f.OwnerID == user.ID
}

/*
PaginateForums scans over the available forums for a user.

Parameters:

- userID: of who is looking
- categories: optional, filter within categories
- pager

The pager Sort accepts a couple of custom values for more advanced sorting:

- by_latest: recently updated posts
- by_threads: thread count
- by_posts: post count
- by_users: user count
*/
func PaginateForums(user *User, categories []string, search *Search, subscribed bool, pager *Pagination) ([]*Forum, error) {
	var (
		fs           = []*Forum{}
		query        = (&Forum{}).Preload()
		wheres       = []string{}
		placeholders = []any{}
	)

	if len(categories) > 0 {
		wheres = append(wheres, "category IN ?")
		placeholders = append(placeholders, categories)
	}

	// Hide explicit forum if user hasn't opted into it.
	if !user.Explicit && !user.IsAdmin {
		wheres = append(wheres, "explicit = false")
	}

	// Hide private forums except for admins and approved users.
	if !user.HasAdminScope(config.ScopeAdminBase) {
		wheres = append(wheres, `
			(
				private IS NOT TRUE
				OR EXISTS (
					SELECT 1
					FROM forum_memberships
					WHERE forum_id = forums.id
					AND user_id = ?
					AND (
						is_moderator IS TRUE
						OR approved IS TRUE
					)
				)
			)`,
		)
		placeholders = append(placeholders, user.ID)
	}

	// Followed forums only? (for the My List category on home page)
	if subscribed {
		wheres = append(wheres, `
			(
				EXISTS (
					SELECT 1
					FROM forum_memberships
					WHERE user_id = ?
					AND forum_id = forums.id
				)
				OR (
					forums.owner_id = ?
					AND (forums.category = '' OR forums.category IS NULL)
				)
			)
		`)
		placeholders = append(placeholders, user.ID, user.ID)
	}

	// Apply their search terms.
	if search != nil {
		for _, term := range search.Includes {
			var ilike = "%" + strings.ToLower(term) + "%"
			wheres = append(wheres, "(fragment ILIKE ? OR title ILIKE ? OR description ILIKE ?)")
			placeholders = append(placeholders, ilike, ilike, ilike)
		}
		for _, term := range search.Excludes {
			var ilike = "%" + strings.ToLower(term) + "%"
			wheres = append(wheres, "(fragment NOT ILIKE ? AND title NOT ILIKE ? AND description NOT ILIKE ?)")
			placeholders = append(placeholders, ilike, ilike, ilike)
		}
	}

	// Filters?
	if len(wheres) > 0 {
		query = query.Where(
			strings.Join(wheres, " AND "),
			placeholders...,
		)
	}

	// Custom SORT parameters.
	switch pager.Sort {
	case "by_followers":
		pager.Sort = `(
			SELECT count(forum_memberships.id)
			FROM forum_memberships
			WHERE forum_memberships.forum_id = forums.id
		) DESC`
	case "by_latest":
		pager.Sort = `(
			SELECT MAX(threads.updated_at)
			FROM threads
			WHERE threads.forum_id = forums.id
		) DESC NULLS LAST`
	case "by_threads":
		pager.Sort = `(
			SELECT count(threads.id)
			FROM threads
			WHERE threads.forum_id = forums.id
		) DESC`
	case "by_posts":
		pager.Sort = `(
			SELECT count(comments.id)
			FROM threads
			JOIN comments ON comments.table_name='threads' AND comments.table_id=threads.id
			WHERE threads.forum_id = forums.id
		) DESC`
	case "by_users":
		pager.Sort = `(
			SELECT count(distinct(users.id))
			FROM threads
			JOIN comments ON comments.table_name='threads' AND comments.table_id=threads.id
			JOIN users ON comments.user_id=users.id
			WHERE threads.forum_id = forums.id
		) DESC`
	}

	query = query.Order(pager.Sort)
	query.Model(&Forum{}).Count(&pager.Total)
	result := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&fs)
	return fs, result.Error
}

// PaginateOwnedForums returns forums the user owns (or all forums to admins).
func PaginateOwnedForums(userID uint64, isAdmin bool, categories []string, search *Search, pager *Pagination) ([]*Forum, error) {
	var (
		fs           = []*Forum{}
		query        = (&Forum{}).Preload()
		wheres       = []string{}
		placeholders = []interface{}{}
	)

	// Users see only their owned forums.
	if !isAdmin {
		wheres = append(wheres, "owner_id = ?")
		placeholders = append(placeholders, userID)
	}

	if len(categories) > 0 {
		wheres = append(wheres, "category IN ?")
		placeholders = append(placeholders, categories)
	}

	// Apply their search terms.
	if search != nil {
		for _, term := range search.Includes {
			var ilike = "%" + strings.ToLower(term) + "%"
			wheres = append(wheres, "(fragment ILIKE ? OR title ILIKE ? OR description ILIKE ?)")
			placeholders = append(placeholders, ilike, ilike, ilike)
		}
		for _, term := range search.Excludes {
			var ilike = "%" + strings.ToLower(term) + "%"
			wheres = append(wheres, "(fragment NOT ILIKE ? AND title NOT ILIKE ? AND description NOT ILIKE ?)")
			placeholders = append(placeholders, ilike, ilike, ilike)
		}
	}

	query = query.Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Order(pager.Sort)

	query.Model(&Forum{}).Count(&pager.Total)
	result := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&fs)
	return fs, result.Error
}

// CreateForum.
func CreateForum(f *Forum) error {
	result := DB.Create(f)
	return result.Error
}

// AllThreadIDs returns all thread IDs belonging to the forum.
func (f *Forum) AllThreadIDs() ([]uint64, error) {
	var threadIDs = []uint64{}
	err := DB.Table(
		"threads",
	).Select(
		"threads.id AS id",
	).Where(
		"forum_id = ?",
		f.ID,
	).Scan(&threadIDs)

	if err.Error != nil {
		return threadIDs, fmt.Errorf("AllThreadIDs(%d): %s", f.ID, err.Error)
	}

	return threadIDs, nil
}

// Delete a forum, which deeply deletes all of its threads and comments.
func (f *Forum) Delete() error {
	// Get all thread IDs.
	threadIDs, err := f.AllThreadIDs()
	if err != nil {
		return fmt.Errorf("delete forum %d: %w", f.ID, err)
	}

	// Delete all comments and threads.
	if len(threadIDs) > 0 {
		// Null out thread refs to parent comments.
		if res := DB.Exec(`
			-- Null out thread refs to parent comments
			UPDATE threads
			SET forum_id = NULL, comment_id = NULL
			WHERE id IN ?
		`, threadIDs); res.Error != nil {
			return fmt.Errorf("delete forum %d: updating threads: %w", f.ID, res.Error)
		}

		// Delete all thread comments
		if res := DB.Exec(`
			DELETE FROM comments
			WHERE table_name='threads'
			AND table_id IN ?;
		`, threadIDs); res.Error != nil {
			return fmt.Errorf("delete forum %d: removing comments: %w", f.ID, res.Error)
		}

		// Remove the threads themselves
		if res := DB.Exec(`
			DELETE FROM threads
			WHERE id IN ?;
		`, threadIDs); res.Error != nil {
			return fmt.Errorf("delete forum %d: removing threads and comments: %w", f.ID, res.Error)
		}
	}

	// Remove forum memberships.
	if res := DB.Exec(`
		DELETE FROM forum_memberships
		WHERE forum_id = ?;
	`, f.ID); res.Error != nil {
		return fmt.Errorf("delete forum %d: removing memberships: %w", f.ID, res.Error)
	}

	// Delete the forum.
	return DB.Delete(f).Error
}

// Save a forum.
func (f *Forum) Save() error {
	return DB.Save(f).Error
}

// CategorizedForum supports the main index page with custom categories.
type CategorizedForum struct {
	Category string
	Forums   []*Forum
}

// CategorizeForums buckets forums into categories for front-end.
func CategorizeForums(fs []*Forum, categories []string) []*CategorizedForum {
	var (
		result = []*CategorizedForum{}
		idxMap = map[string]int{}
	)

	// Forum Browse page: we are not grouping by categories but still need at least one.
	if len(categories) == 0 {
		return []*CategorizedForum{
			{
				Forums: fs,
			},
		}
	}

	// Initialize the result set.
	for i, category := range categories {
		result = append(result, &CategorizedForum{
			Category: category,
			Forums:   []*Forum{},
		})
		idxMap[category] = i
	}

	// Bucket the forums into their categories.
	for _, forum := range fs {
		category := forum.Category
		if category == "" {
			continue
		}
		idx := idxMap[category]
		result[idx].Forums = append(result[idx].Forums, forum)
	}

	// Remove any blank categories with no boards.
	var filtered = []*CategorizedForum{}
	for _, forum := range result {
		if len(forum.Forums) == 0 {
			continue
		}
		filtered = append(filtered, forum)
	}

	return filtered
}
