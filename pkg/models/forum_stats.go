package models

import (
	"github.com/cuvou/gosocial/pkg/log"
)

// ForumStatistics queries for forum-level statistics.
type ForumStatistics struct {
	RecentThread *Thread
	RecentPost   *Comment // latest post on the recent thread
	Threads      uint64
	Posts        uint64
	Users        uint64
}

type ForumStatsMap map[uint64]*ForumStatistics

// MapForumStatistics looks up statistics for a set of forums.
func MapForumStatistics(forums []*Forum) ForumStatsMap {
	var (
		result = ForumStatsMap{}
		IDs    = []uint64{}
	)

	// Collect forum IDs and initialize the map.
	for _, forum := range forums {
		IDs = append(IDs, forum.ID)
		result[forum.ID] = &ForumStatistics{}
	}

	// Gather all the statistics.
	result.generateThreadCount(IDs)
	result.generatePostCount(IDs)
	result.generateUserCount(IDs)
	result.generateRecentThreads(IDs)
	result.generateRecentPosts(IDs)

	return result
}

// Has stats for this thread? (we should..)
func (ts ForumStatsMap) Has(threadID uint64) bool {
	_, ok := ts[threadID]
	return ok
}

// Get thread stats.
func (ts ForumStatsMap) Get(threadID uint64) *ForumStatistics {
	if stats, ok := ts[threadID]; ok {
		return stats
	}
	return nil
}

// Compute the count of threads in each of the forum 'IDs'.
func (ts ForumStatsMap) generateThreadCount(IDs []uint64) {
	// Hold the result of the count/group by query.
	type group struct {
		ID      uint64
		Threads uint64
	}
	var groups = []group{}

	// Count comments grouped by thread IDs.
	err := DB.Table(
		"threads",
	).Select(
		"forum_id AS id, count(id) AS threads",
	).Where(
		"forum_id IN ?",
		IDs,
	).Group("forum_id").Scan(&groups)

	if err.Error != nil {
		log.Error("MapForumStatistics: SQL error: %s", err.Error)
	}

	// Map the results in.
	for _, row := range groups {
		if stats, ok := ts[row.ID]; ok {
			stats.Threads = row.Threads
		}
	}
}

// Compute the count of all posts in each of the forum 'IDs'.
func (ts ForumStatsMap) generatePostCount(IDs []uint64) {
	type group struct {
		ID    uint64
		Posts uint64
	}
	var groups = []group{}

	err := DB.Table(
		"comments",
	).Joins(
		"JOIN threads ON (table_name = 'threads' AND table_id = threads.id)",
	).Joins(
		"JOIN forums ON (threads.forum_id = forums.id)",
	).Select(
		"forums.id AS id, count(comments.id) AS posts",
	).Where(
		`table_name = 'threads' AND EXISTS (
			SELECT 1
			FROM threads
			WHERE table_id = threads.id
			AND threads.forum_id IN ?
		)`,
		IDs,
	).Group("forums.id").Scan(&groups)

	if err.Error != nil {
		log.Error("SQL error collecting posts for forum: %s", err.Error)
	}

	// Map the results in.
	for _, row := range groups {
		if stats, ok := ts[row.ID]; ok {
			stats.Posts = row.Posts
		}
	}
}

// Compute the count of all users in each of the forum 'IDs'.
func (ts ForumStatsMap) generateUserCount(IDs []uint64) {
	type group struct {
		ForumID uint64
		Users   uint64
	}
	var groups = []group{}

	err := DB.Table(
		"comments",
	).Joins(
		"JOIN threads ON (table_name = 'threads' AND table_id = threads.id)",
	).Joins(
		"JOIN forums ON (threads.forum_id = forums.id)",
	).Select(
		"forums.id AS forum_id, count(distinct(comments.user_id)) AS users",
	).Where(
		"forums.id IN ?",
		IDs,
	).Group("forums.id").Scan(&groups)

	if err.Error != nil {
		log.Error("SQL error collecting users for forum: %s", err.Error)
	}

	// Map the results in.
	for _, row := range groups {
		if stats, ok := ts[row.ForumID]; ok {
			stats.Users = row.Users
		}
	}
}

// Compute the recent threads for each of the forum 'IDs'.
func (ts ForumStatsMap) generateRecentThreads(IDs []uint64) {
	var threadIDs = []map[string]interface{}{}
	err := DB.Table(
		"threads",
	).Select(
		"forum_id, id AS thread_id, updated_at",
	).Where(
		`updated_at = (SELECT MAX(updated_at)
			               FROM threads t2
						   WHERE threads.forum_id = t2.forum_id)
			AND threads.forum_id IN ?`,
		IDs,
	).Order(
		"updated_at desc",
	).Scan(&threadIDs)

	if err.Error != nil {
		log.Error("Getting most recent thread IDs: %s", err.Error)
	}

	// Map them easier.
	var (
		threadForumMap = map[uint64]uint64{}
		allThreadIDs   = []uint64{}
	)
	for _, row := range threadIDs {
		if row["thread_id"] == nil || row["forum_id"] == nil {
			continue
		}

		var (
			threadID = uint64(row["thread_id"].(int64))
			forumID  = uint64(row["forum_id"].(int64))
		)

		allThreadIDs = append(allThreadIDs, threadID)
		threadForumMap[threadID] = forumID
	}

	// Select and map these threads in.
	if threadMap, err := GetThreads(allThreadIDs); err == nil {
		for threadID, thread := range threadMap {
			if forumID, ok := threadForumMap[threadID]; ok {
				if stats, ok := ts[forumID]; ok {
					stats.RecentThread = thread
				}
			}
		}
	}
}

// Compute the recent post on each recent thread of each forum.
func (ts ForumStatsMap) generateRecentPosts(IDs []uint64) {
	// We already have the RecentThread of each forum - map these Thread IDs to recent comments.
	var (
		threadIDs      = []uint64{}
		threadStatsMap = map[uint64]*ForumStatistics{}
	)
	for _, stats := range ts {
		if stats.RecentThread != nil {
			threadIDs = append(threadIDs, stats.RecentThread.ID)
			threadStatsMap[stats.RecentThread.ID] = stats
		}
	}

	// The newest posts in these threads.
	type scanner struct {
		ThreadID  uint64
		CommentID uint64
	}
	var scan []scanner
	err := DB.Table(
		"comments",
	).Select(
		"table_id AS thread_id, id AS comment_id",
	).Where(
		`table_name='threads' AND table_id IN ?
		AND updated_at = (SELECT MAX(updated_at)
						  FROM comments c2
						  WHERE c2.table_name=comments.table_name
						  AND c2.table_id=comments.table_id
		)`,
		threadIDs,
	).Order(
		"updated_at desc",
	).Scan(&scan)
	if err.Error != nil {
		log.Error("Getting most recent post IDs: %s", err.Error)
	}

	// Gather the ThreadID:CommentID map.
	var (
		commentIDs      = []uint64{}
		commentStatsMap = map[uint64]*ForumStatistics{}
	)
	for _, row := range scan {
		if stats, ok := threadStatsMap[row.ThreadID]; ok {
			commentStatsMap[row.CommentID] = stats
			commentIDs = append(commentIDs, row.CommentID)
		}
	}

	// Select all these comments and map them in.
	if commentMap, err := GetComments(commentIDs); err == nil {
		for commentId, comment := range commentMap {
			if stats, ok := commentStatsMap[commentId]; ok {
				stats.RecentPost = comment
			}
		}
	}
}
