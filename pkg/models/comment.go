package models

import (
	"errors"
	"math"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/log"
	"gorm.io/gorm"
)

// Comment table - in forum threads, on profiles or photos, etc.
type Comment struct {
	ID        uint64 `gorm:"primaryKey"`
	TableName string `gorm:"index:idx_comment_composite"`
	TableID   uint64 `gorm:"index:idx_comment_composite"`
	UserID    uint64 `gorm:"index"`
	User      User   `json:"-"`
	Message   string
	CreatedAt time.Time `gorm:"index"`
	UpdatedAt time.Time
}

// CommentableTables are the set of table names that allow comments (via the
// generic "/comments" URI which accepts a table_name param)
var CommentableTables = map[string]interface{}{
	"photos":  nil,
	"threads": nil,
}

// SubscribableTables are the set of table names that allow notification subscriptions.
var SubscribableTables = map[string]interface{}{
	"photos":  nil,
	"threads": nil,
}

// DummyCommentUnavailableOP is a mostly-empty comment that is injected on forum threads when the
// OP comment on the thread is not available due to block lists.
var DummyCommentUnavailableOP = &Comment{
	Message: "*The first comment that started this thread is not available for display.*\n\n" +
		"*The most common cause of this is that one of you has blocked the other on the website, or " +
		"because the original author may have deactivated their [gosocial] account.*",
}

// Preload related tables for the forum (classmethod).
func (c *Comment) Preload() *gorm.DB {
	return DB.Preload("User.ProfilePhoto")
}

// GetComment by ID.
func GetComment(id uint64) (*Comment, error) {
	c := &Comment{}
	result := c.Preload().First(&c, id)
	return c, result.Error
}

// GetComments queries a set of comment IDs and returns them mapped.
func GetComments(IDs []uint64) (map[uint64]*Comment, error) {
	var (
		mt = map[uint64]*Comment{}
		ts = []*Comment{}
	)

	result := (&Comment{}).Preload().Where("id IN ?", IDs).Find(&ts)
	for _, row := range ts {
		mt[row.ID] = row
	}

	return mt, result.Error
}

// AddComment about anything.
func AddComment(user *User, tableName string, tableID uint64, message string) (*Comment, error) {
	c := &Comment{
		TableName: tableName,
		TableID:   tableID,
		User:      *user,
		Message:   message,
	}
	result := DB.Create(c)
	return c, result.Error
}

// CountCommentsByUser returns the total number of comments written by a user.
func CountCommentsByUser(user *User, tableName string) int64 {
	var count int64
	result := DB.Where(
		"table_name = ? AND user_id = ?",
		tableName, user.ID,
	).Model(&Comment{}).Count(&count)
	if result.Error != nil {
		log.Error("CountCommentsByUser(%d): %s", user.ID, result.Error)
	}
	return count
}

// CountCommentPhotosByUser returns the count of comments with photo attachments.
func CountCommentPhotosByUser(user *User) int64 {
	var count int64
	result := DB.Where(
		"user_id = ?",
		user.ID,
	).Model(&CommentPhoto{}).Count(&count)
	if result.Error != nil {
		log.Error("CountCommentPhotosByUser(%d): %s", user.ID, result.Error)
	}
	return count
}

// CountCommentsReceived returns the total number of comments received on a user's photos.
func CountCommentsReceived(user *User) int64 {
	var count int64
	DB.Model(&Comment{}).Joins(
		"LEFT OUTER JOIN photos ON (comments.table_name = 'photos' AND comments.table_id = photos.id)",
	).Where(
		"comments.table_name = 'photos' AND photos.user_id = ?",
		user.ID,
	).Count(&count)
	return count
}

// PaginateComments provides a page of comments on something.
//
// Note: noBlockLists is to facilitate user-owned forums, where forum owners/moderators should override the block lists
// and retain full visibility into all user comments on their forum. Default/recommended is to leave it false, where
// the user's block list filters the view.
func PaginateComments(user *User, tableName string, tableID uint64, noBlockLists bool, pager *Pagination) ([]*Comment, error) {
	var (
		cs           = []*Comment{}
		query        = (&Comment{}).Preload()
		wheres       = []string{}
		placeholders = []any{}
	)

	wheres = append(wheres, "table_name = ? AND table_id = ?")
	placeholders = append(placeholders, tableName, tableID)

	if !noBlockLists {
		// Blocking user IDs?
		bw, bp := BlockedUserSubquery("user_id", user.ID)
		wheres = append(wheres, bw)
		placeholders = append(placeholders, bp...)
	}

	// Don't show comments from banned or disabled accounts.
	wheres = append(wheres, `
		EXISTS (
			SELECT 1
			FROM users
			WHERE users.id = comments.user_id
			AND users.status = 'active'
		)
	`)

	query = query.Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Order(pager.Sort)

	query.Model(&Comment{}).Count(&pager.Total)
	result := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&cs)

	// Inject user relationships into these comments' authors.
	SetUserRelationshipsInComments(user, cs)

	return cs, result.Error
}

// FindPageByComment finds out what page a comment ID exists on for the current user, taking into
// account their block lists and comment visibility.
//
// Note: the comments are assumed ordered by created_at asc.
func FindPageByComment(user *User, comment *Comment, pageSize int) (int, error) {
	var (
		allCommentIDs []uint64
		wheres        = []string{}
		placeholders  = []interface{}{}
	)

	// Get the complete set of comment IDs that this comment is on a thread of.
	wheres = append(wheres, "comments.table_name = ? AND comments.table_id = ?")
	placeholders = append(placeholders, comment.TableName, comment.TableID)

	// Blocking user IDs?
	bw, bp := BlockedUserSubquery("comments.user_id", user.ID)
	wheres = append(wheres, bw)
	placeholders = append(placeholders, bp...)

	// Filter out inactive users.
	wheres = append(wheres, "users.status = ?")
	placeholders = append(placeholders, UserStatusActive)

	result := DB.Table(
		"comments",
	).Select(
		"comments.id",
	).Joins(
		"JOIN users ON (users.id = comments.user_id)",
	).Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Order("comments.created_at asc").Scan(&allCommentIDs)
	if result.Error != nil {
		return 0, result.Error
	}

	// Scan the comment thread to find it.
	for i, cid := range allCommentIDs {
		if cid == comment.ID {
			var page = int(math.Ceil(float64(i) / float64(pageSize)))

			// If the comment index is an equal multiple of the page size
			// (e.g. comment #20 is the 1st comment on page 2, since 0-19 is page 1),
			// account for an off-by-one error.
			if i%pageSize == 0 {
				page++
			}

			if page == 0 {
				page = 1
			}
			return page, nil
		}
	}

	return -1, errors.New("comment not visible to current user")
}

// ListComments returns a complete set of comments without paging.
func ListComments(user *User, tableName string, tableID uint64, sort string) ([]*Comment, error) {
	var (
		cs           []*Comment
		wheres       = []string{}
		placeholders = []interface{}{}
	)

	if sort == "" {
		sort = "created_at asc"
	}

	wheres = append(wheres, "table_name = ? AND table_id = ?")
	placeholders = append(placeholders, tableName, tableID)

	// Blocking user IDs?
	bw, bp := BlockedUserSubquery("user_id", user.ID)
	wheres = append(wheres, bw)
	placeholders = append(placeholders, bp...)

	// Don't show comments from banned or disabled accounts.
	wheres = append(wheres, `
		EXISTS (
			SELECT 1
			FROM users
			WHERE users.id = comments.user_id
			AND users.status = 'active'
		)
	`)

	result := (&Comment{}).Preload().Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Order(sort).Find(&cs)
	return cs, result.Error
}

// Save a comment.
func (c *Comment) Save() error {
	return DB.Save(c).Error
}

// Delete a comment.
func (c *Comment) Delete() error {
	return DB.Delete(c).Error
}

// IsEdited returns if a comment was reasonably edited after it was created.
func (c *Comment) IsEdited() bool {
	return c.UpdatedAt.Sub(c.CreatedAt) > 5*time.Second
}

// MapTopLevelComments will check whether any of your comments are the top-level post on a forum thread.
//
// The result map only contains a comment ID if it was the top-level of a Thread.
//
// Note: not for front-end template use.
func MapTopLevelComments(commentIDs []uint64) map[uint64]*Thread {
	if len(commentIDs) == 0 {
		return nil
	}

	var (
		result  = map[uint64]*Thread{}
		threads []*Thread
	)

	// Look up threads that have these comments.
	DB.Model(&Thread{}).Where(
		"comment_id IN ?",
		commentIDs,
	).Find(&threads)
	for _, thread := range threads {
		result[thread.CommentID] = thread
	}

	return result
}

type CommentCountMap map[uint64]int64

// MapCommentCounts collects total numbers of comments over a set of table IDs. Returns a
// map of table ID (uint64) to comment counts for each (int64).
func MapCommentCounts(tableName string, tableIDs []uint64) CommentCountMap {
	var result = CommentCountMap{}

	// Initialize the result set.
	for _, id := range tableIDs {
		result[id] = 0
	}

	// Hold the result of the grouped count query.
	type group struct {
		ID       uint64
		Comments int64
	}
	var groups = []group{}

	// Map the counts of comments to each of these IDs.
	if res := DB.Table(
		"comments",
	).Select(
		"table_id AS id, count(id) AS comments",
	).Where(
		"table_name = ? AND table_id IN ?",
		tableName, tableIDs,
	).Group("table_id").Scan(&groups); res.Error != nil {
		log.Error("MapCommentCounts: count query: %s", res.Error)
	}

	// Map the counts back in.
	for _, row := range groups {
		result[row.ID] = row.Comments
	}

	return result
}

// Get a comment count for the given table ID from the map.
func (cc CommentCountMap) Get(id uint64) int64 {
	if value, ok := cc[id]; ok {
		return value
	}
	return 0
}
