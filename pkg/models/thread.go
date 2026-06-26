package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/redis"
	"gorm.io/gorm"
)

// Thread table - a post within a Forum.
type Thread struct {
	ID        uint64 `gorm:"primaryKey"`
	ForumID   uint64 `gorm:"index"`
	Forum     Forum
	Pinned    bool `gorm:"index"`
	Explicit  bool `gorm:"index"`
	NoReply   bool
	Title     string
	CommentID uint64  `gorm:"index"`
	Comment   Comment // first comment of the thread
	PollID    *uint64 `gorm:"poll_id"`
	Poll      Poll    // if the thread has a poll attachment
	Views     uint64
	CreatedAt time.Time
	UpdatedAt time.Time
}

// Preload related tables for the forum (classmethod).
func (f *Thread) Preload() *gorm.DB {
	return DB.Preload("Forum").Preload("Comment.User.ProfilePhoto").Preload("Poll")
}

// GetThread by ID.
func GetThread(id uint64) (*Thread, error) {
	t := &Thread{}
	result := t.Preload().First(&t, id)
	return t, result.Error
}

// GetThreads queries a set of thread IDs and returns them mapped.
func GetThreads(IDs []uint64) (map[uint64]*Thread, error) {
	var (
		mt           = map[uint64]*Thread{}
		ts           = []*Thread{}
		wheres       = []string{"threads.id IN ?"}
		placeholders = []interface{}{IDs}
	)

	// Don't show threads from banned or disabled accounts.
	wheres = append(wheres, `
		EXISTS (
			SELECT 1
			FROM users
			WHERE users.id = comments.user_id
			AND users.status = 'active'
		)
	`)

	result := (&Thread{}).Preload().Joins(
		"LEFT OUTER JOIN comments ON (comments.id = threads.comment_id)",
	).Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Find(&ts)
	for _, row := range ts {
		mt[row.ID] = row
	}

	return mt, result.Error
}

// GetThreadsAsUser queries a set of thread IDs and returns them mapped, taking blocklists into consideration.
func GetThreadsAsUser(currentUser *User, IDs []uint64) (map[uint64]*Thread, error) {
	var (
		mt           = map[uint64]*Thread{}
		ts           = []*Thread{}
		wheres       = []string{"threads.id IN ?"}
		placeholders = []interface{}{IDs}
	)

	// Blocking user IDs?
	bw, bp := BlockedUserSubquery("comments.user_id", currentUser.ID)
	wheres = append(wheres, bw)
	placeholders = append(placeholders, bp...)

	// Don't show threads from banned or disabled accounts.
	wheres = append(wheres, `
		EXISTS (
			SELECT 1
			FROM users
			WHERE users.id = comments.user_id
			AND users.status = 'active'
		)
	`)

	result := (&Thread{}).Preload().Joins(
		"LEFT OUTER JOIN comments ON (comments.id = threads.comment_id)",
	).Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Find(&ts)
	for _, row := range ts {
		mt[row.ID] = row
	}

	return mt, result.Error
}

// CreateThread creates a new thread with proper Comment structure.
func CreateThread(user *User, forumID uint64, title, message string, pinned, explicit, noReply bool) (*Thread, error) {
	thread := &Thread{
		ForumID:  forumID,
		Title:    title,
		Pinned:   pinned,
		Explicit: explicit,
		NoReply:  noReply && user.IsAdmin,
		Comment: Comment{
			User:    *user,
			Message: message,
		},
	}

	log.Error("CreateThread: Going to post %+v", thread)

	// Create the thread & comment first...
	result := DB.Create(thread)
	if result.Error != nil {
		return nil, result.Error
	}

	// Fill out the Comment with proper reverse foreign keys.
	thread.Comment.TableName = "threads"
	thread.Comment.TableID = thread.ID
	log.Error("Saving updated comment: %+v", thread)
	result = DB.Save(&thread.Comment)
	return thread, result.Error
}

// Pages returns the number of pages in the thread - also useful to find out
// what is the final page number that has any posts.
func (t *Thread) Pages() int {
	// How many posts total?
	var postCount int64
	var query = DB.Table(
		"comments",
	).Select(
		"count(id) AS count",
	).Where(
		"table_name = 'threads' AND table_id = ?",
		t.ID,
	).Count(&postCount)
	if query.Error != nil {
		log.Error("SQL error getting post count for thread %d: %s", t.ID, query.Error)
	}

	// Return what the Paginator would say is the inclusive page count.
	return Pagination{
		PerPage: config.PageSizeThreadList,
		Total:   postCount,
	}.Pages()
}

// Move the thread to another forum.
func (t *Thread) Move(forum *Forum) error {
	res := DB.Exec(`
		UPDATE threads
		SET forum_id = ?
		WHERE id = ?
	`, forum.ID, t.ID)
	return res.Error
}

// Reply to a thread, adding an additional comment.
func (t *Thread) Reply(user *User, message string) (*Comment, error) {
	// Save the thread on reply, updating its timestamp.
	if err := t.Save(); err != nil {
		log.Error("Thread.Reply: couldn't ping UpdatedAt on thread: %s", err)
	}

	return AddComment(user, "threads", t.ID, message)
}

// DeleteReply removes a comment from a thread. If it is the primary comment, deletes the whole thread.
func (t *Thread) DeleteReply(comment *Comment) error {
	// Sanity check that this reply is one of ours.
	if !(comment.TableName == "threads" && comment.TableID == t.ID) {
		return errors.New("that comment doesn't belong to this thread")
	}

	// Revoke any notifications sent to subscribers when this reply was first created.
	if err := RemoveAlsoPostedNotification(t, comment.ID); err != nil {
		log.Error("Thread.DeleteReply: RemoveAlsoPostedNotification: %s", err)
	}

	// Is this the primary comment that started the thread? If so, delete the whole thread.
	if comment.ID == t.CommentID {
		log.Error("DeleteReply(%d): this is the parent comment of a thread (%d '%s'), remove the whole thread", comment.ID, t.ID, t.Title)
		return t.Delete()
	}

	// Remove just this comment.
	return comment.Delete()
}

// PinnedThreads returns all pinned threads in a forum (there should generally be few of these).
func PinnedThreads(forum *Forum) ([]*Thread, error) {
	var (
		ts    = []*Thread{}
		query = (&Thread{}).Preload().Where(
			"forum_id = ? AND pinned IS TRUE",
			forum.ID,
		).Order("updated_at desc")
	)

	result := query.Find(&ts)
	return ts, result.Error
}

// CountThreadsByUser returns the total number of forum threads started by a user.
func CountThreadsByUser(user *User) int64 {
	var count int64
	result := DB.Joins(
		"JOIN comments ON (comments.id = threads.comment_id)",
	).Where(
		"comments.user_id = ?",
		user.ID,
	).Model(&Thread{}).Count(&count)
	if result.Error != nil {
		log.Error("CountThreadsByUser(%d): %s", user.ID, result.Error)
	}
	return count
}

// PaginateThreads provides a forum index view of posts, minus pinned posts.
func PaginateThreads(user *User, forum *Forum, pager *Pagination) ([]*Thread, error) {
	var (
		ts           = []*Thread{}
		query        = (&Thread{}).Preload()
		wheres       = []string{}
		placeholders = []interface{}{}
	)

	// Always filters.
	wheres = append(wheres, "forum_id = ? AND pinned IS NOT TRUE")
	placeholders = append(placeholders, forum.ID)

	// If the user hasn't opted in for Explicit, hide NSFW threads.
	if !user.Explicit && !user.IsAdmin {
		wheres = append(wheres, "explicit IS NOT TRUE")
	}

	// Don't show threads from banned or disabled accounts.
	wheres = append(wheres, `
		EXISTS (
			SELECT 1
			FROM users
			WHERE users.id = comments.user_id
			AND users.status = 'active'
		)
	`)

	query = query.Joins(
		"LEFT OUTER JOIN comments ON (comments.id = threads.comment_id)",
	).Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Order(pager.Sort)

	query.Model(&Thread{}).Count(&pager.Total)
	result := query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&ts)

	// Inject user relationships into these threads' comments' users.
	SetUserRelationshipsInThreads(user, ts)

	return ts, result.Error
}

// View a thread, incrementing its View count but not its UpdatedAt.
// Debounced with a Redis key.
func (t *Thread) View(userID uint64) error {
	// Debounce this.
	var redisKey = fmt.Sprintf(config.ThreadViewDebounceRedisKey, userID, t.ID)
	if redis.Exists(redisKey) {
		return nil
	}
	redis.Set(redisKey, nil, config.ThreadViewDebounceCooldown)

	return DB.Model(&Thread{}).Where(
		"id = ?",
		t.ID,
	).Updates(map[string]interface{}{
		"views":      t.Views + 1,
		"updated_at": t.UpdatedAt,
	}).Error
}

// Save a thread, updating its timestamp.
func (t *Thread) Save() error {
	return DB.Save(t).Error
}

// SaveModeration change to a thread (e.g. its pinned, locked or explicit status) without updating its timestamp.
func (t *Thread) SaveModeration() error {
	return DB.Exec(
		`
			UPDATE threads
			SET no_reply = ?, pinned = ?, explicit = ?
			WHERE id = ?
		`,
		t.NoReply,
		t.Pinned,
		t.Explicit,
		t.ID,
	).Error
}

// Delete a thread and all of its comments.
func (t *Thread) Delete() error {
	// Unlink the parent comment from the thread to resolve a foreign key constraint in Postgres.
	if result := DB.Model(&Thread{}).Where("id = ?", t.ID).Update("comment_id", nil); result.Error != nil {
		return fmt.Errorf("Thread.Delete: couldn't unlink parent comment: %s", result.Error)
	}

	// Remove all comments.
	if result := DB.Where(
		"table_name = ? AND table_id = ?",
		"threads", t.ID,
	).Delete(&Comment{}); result.Error != nil {
		return fmt.Errorf("deleting comments for thread: %s", result.Error)
	}

	// Remove any polls.
	if t.PollID != nil && *t.PollID > 0 {
		if result := DB.Exec(
			"UPDATE threads SET poll_id=NULL WHERE id = ?",
			t.ID,
		); result.Error != nil {
			return fmt.Errorf("nulling poll ID for thread: %s", result.Error)
		}

		if result := DB.Exec(
			"DELETE FROM polls WHERE id = ?",
			t.PollID,
		); result.Error != nil {
			return fmt.Errorf("deleting poll for thread: %s", result.Error)
		}
	}

	// Remove the thread itself.
	return DB.Delete(t).Error
}

// ThreadStatistics queries for reply/view count for threads.
type ThreadStatistics struct {
	Replies uint64
	Views   uint64
}

type ThreadStatsMap map[uint64]*ThreadStatistics

// MapThreadStatistics looks up statistics for a set of threads.
func MapThreadStatistics(threads []*Thread) ThreadStatsMap {
	var (
		result = ThreadStatsMap{}
		IDs    = []uint64{}
	)

	// Collect thread IDs and initialize the map.
	for _, thread := range threads {
		IDs = append(IDs, thread.ID)
		result[thread.ID] = &ThreadStatistics{
			Views: thread.Views,
		}
	}

	// Hold the result of the count/group by query.
	type group struct {
		ID      uint64
		Replies uint64
	}
	var groups = []group{}

	// Count comments grouped by thread IDs.
	err := DB.Table(
		"comments",
	).Select(
		"table_id AS id, count(id) AS replies",
	).Where(
		"table_name = ? AND table_id IN ?",
		"threads", IDs,
	).Group("table_id").Scan(&groups)

	if err != nil {
		log.Error("MapThreadStatistics: SQL error: %s", err.Error)
	}

	// Map the results in.
	for _, row := range groups {
		log.Error("Got row: %+v", row)
		if stats, ok := result[row.ID]; ok {
			stats.Replies = row.Replies

			// Remove the OG comment from the count.
			if stats.Replies > 0 {
				stats.Replies--
			}
		}
	}

	return result
}

// Has stats for this thread? (we should..)
func (ts ThreadStatsMap) Has(threadID uint64) bool {
	_, ok := ts[threadID]
	return ok
}

// Get thread stats.
func (ts ThreadStatsMap) Get(threadID uint64) *ThreadStatistics {
	if stats, ok := ts[threadID]; ok {
		return stats
	}
	return nil
}

type ForumCommentThreadMap map[uint64]*Thread

// MapForumCommentThreads maps a set of comments to the forum thread they are posted on.
func MapForumCommentThreads(comments []*Comment) (ForumCommentThreadMap, error) {
	var (
		result    = ForumCommentThreadMap{}
		threadIDs = []uint64{}
	)

	for _, com := range comments {
		if com.TableName != "threads" {
			continue
		}
		threadIDs = append(threadIDs, com.TableID)
	}

	if len(threadIDs) == 0 {
		return result, nil
	}

	threads, err := GetThreads(threadIDs)
	if err != nil {
		return nil, err
	}

	for _, com := range comments {
		if thr, ok := threads[com.TableID]; ok {
			result[com.ID] = thr
		}
	}

	return result, nil
}

func (m ForumCommentThreadMap) Get(commentID uint64) *Thread {
	return m[commentID]
}
