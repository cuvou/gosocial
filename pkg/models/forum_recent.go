package models

import (
	"sort"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
)

// RecentPost drives the "Forums / Newest" page - carrying all forum comments
// on all threads sorted by date.
type RecentPost struct {
	CommentID uint64
	ThreadID  uint64
	ForumID   uint64
	UpdatedAt time.Time
	Thread    *Thread
	Comment   *Comment
	Forum     *Forum
}

// PaginateRecentPosts returns all of the comments on a forum paginated.
func PaginateRecentPosts(user *User, categories []string, subscribed, allComments bool, pager *Pagination) ([]*RecentPost, error) {
	var (
		result = []*RecentPost{}

		// Separate the WHERE clauses that involve forums/threads from the ones
		// that involve comments. Rationale: if the user is getting a de-duplicated
		// thread view, we'll end up running two queries - one to get all threads and
		// another to get the latest comments, and the WHERE clauses need to be separate.
		wheres         = []string{}
		placeholders   = []any{}
		comment_wheres = []string{"table_name = 'threads'"}
		comment_ph     = []any{}
	)

	if len(categories) > 0 {
		wheres = append(wheres, "forums.category IN ?")
		placeholders = append(placeholders, categories)
	}

	// Hide explicit forum if user hasn't opted into it.
	if !user.Explicit && !user.IsAdmin {
		wheres = append(wheres, "forums.explicit = false")
	}

	// Private forums.
	if !user.HasAdminScope(config.ScopeAdminBase) {
		wheres = append(wheres, `
			(
				forums.private IS NOT TRUE
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

	// Forums I follow?
	if subscribed {
		wheres = append(wheres, `
			EXISTS (
				SELECT 1
				FROM forum_memberships
				WHERE user_id = ?
				AND forum_id = forums.id
			)
		`)
		placeholders = append(placeholders, user.ID)
	}

	// Blocking user IDs?
	bw, bp := BlockedUserSubquery("comments.user_id", user.ID)
	comment_wheres = append(comment_wheres, bw)
	comment_ph = append(comment_ph, bp...)

	// Don't show comments from banned or disabled accounts.
	comment_wheres = append(comment_wheres, `
		EXISTS (
			SELECT 1
			FROM users
			WHERE users.id = comments.user_id
			AND users.status = 'active'
		)
	`)

	// Get the page of recent forum comment IDs of all time.
	var scan NewestForumPostsScanner

	// Deduplicate forum threads: if one thread is BLOWING UP with replies, we should only
	// mention the thread once and show the newest comment so it doesn't spam the whole page.
	if config.Current.Database.IsPostgres && !allComments {
		// Note: only Postgres supports this function (SELECT DISTINCT ON).
		if res, err := ScanLatestForumCommentsPerThread(wheres, comment_wheres, placeholders, comment_ph, pager); err != nil {
			return nil, err
		} else {
			scan = res
		}
	} else {
		// SQLite/non-Postgres doesn't support DISTINCT ON, this is the old query which
		// shows objectively all comments and a popular thread may dominate the page.
		if res, err := ScanLatestForumCommentsAll(wheres, comment_wheres, placeholders, comment_ph, pager); err != nil {
			return nil, err
		} else {
			scan = res
		}
	}

	// Ingest the results.
	var (
		commentIDs   = []uint64{} // collect distinct IDs
		threadIDs    = []uint64{}
		forumIDs     = []uint64{}
		seenComments = map[uint64]interface{}{} // deduplication
		seenThreads  = map[uint64]interface{}{}
		seenForums   = map[uint64]interface{}{}
		mapCommentRC = map[uint64]*RecentPost{} // map commentID to result
	)
	for _, row := range scan {
		// Upsert the result set.
		var rp *RecentPost
		if existing, ok := mapCommentRC[row.CommentID]; ok {
			rp = existing
		} else {
			rp = &RecentPost{
				CommentID: row.CommentID,
			}
			mapCommentRC[row.CommentID] = rp
			result = append(result, rp)
		}

		// Got a thread ID?
		if row.ThreadID != nil {
			rp.ThreadID = *row.ThreadID
			if _, ok := seenThreads[rp.ThreadID]; !ok {
				seenThreads[rp.ThreadID] = nil
				threadIDs = append(threadIDs, rp.ThreadID)
			}

		}

		// Got a forum ID?
		if row.ForumID != nil {
			rp.ForumID = *row.ForumID
			if _, ok := seenForums[rp.ForumID]; !ok {
				seenForums[rp.ForumID] = nil
				forumIDs = append(forumIDs, rp.ForumID)
			}
		}

		// Collect distinct comment IDs.
		if _, ok := seenComments[rp.CommentID]; !ok {
			seenComments[rp.CommentID] = nil
			commentIDs = append(commentIDs, rp.CommentID)
		}
	}

	// Load all of the distinct comments, threads and forums.
	var (
		comments = map[uint64]*Comment{}
		threads  = map[uint64]*Thread{}
		forums   = map[uint64]*Forum{}
	)

	if len(commentIDs) > 0 {
		comments, _ = GetComments(commentIDs)
	}
	if len(threadIDs) > 0 {
		threads, _ = GetThreadsAsUser(user, threadIDs)
	}
	if len(forumIDs) > 0 {
		forums, _ = GetForums(forumIDs)
	}

	// Collect comments so we can inject UserRelationships in efficiently.
	var (
		coms = []*Comment{}
		thrs = []*Thread{}
	)

	// Merge all the objects back in.
	for _, rc := range result {
		if com, ok := comments[rc.CommentID]; ok {
			rc.Comment = com
			coms = append(coms, com)
		}

		if thr, ok := threads[rc.ThreadID]; ok {
			rc.Thread = thr
			thrs = append(thrs, thr)
		} else {
			log.Error("RecentPosts: didn't find thread ID %d in map!", rc.ThreadID)

			// Create a dummy placeholder Thread (e.g.: the thread originator has been
			// banned or disabled, but the thread summary is shown on the new comment view)
			rc.Thread = &Thread{
				Comment: Comment{
					Message: "[unavailable]",
				},
			}
		}

		// Is the new comment unavailable? (e.g. blocked, banned, disabled)
		if rc.Comment == nil {
			rc.Comment = &Comment{
				Message: "[unavailable]",
			}
		}

		if f, ok := forums[rc.ForumID]; ok {
			rc.Forum = f
		}
	}

	// Inject user relationships into all comment users now.
	SetUserRelationshipsInComments(user, coms)
	SetUserRelationshipsInThreads(user, thrs)

	return result, nil
}

// NewestForumPosts collects the IDs of the latest forum posts.
type NewestForumPosts struct {
	CommentID uint64
	ThreadID  *uint64
	ForumID   *uint64
	UpdatedAt time.Time
}

type NewestForumPostsScanner []NewestForumPosts

// ScanLatestForumCommentsAll returns a scan of Newest forum posts containing ALL comments, which may
// include runs of 'duplicate' forum threads if a given thread was commented on rapidly. This is the classic
// 'Newest' tab behavior, showing just ALL forum comments by newest.
func ScanLatestForumCommentsAll(wheres, comment_wheres []string, placeholders, comment_ph []interface{}, pager *Pagination) (NewestForumPostsScanner, error) {
	var scan NewestForumPostsScanner

	// This one is all one joined query so join the wheres/placeholders.
	wheres = append(wheres, comment_wheres...)
	placeholders = append(placeholders, comment_ph...)

	// SQLite/non-Postgres doesn't support DISTINCT ON, this is the old query which
	// shows objectively all comments and a popular thread may dominate the page.
	query := DB.Table("comments").Select(
		`comments.id AS comment_id,
			 threads.id AS thread_id,
			 forums.id AS forum_id,
			 comments.updated_at AS updated_at`,
	).Joins(
		"LEFT OUTER JOIN threads ON (table_name = 'threads' AND table_id = threads.id)",
	).Joins(
		"LEFT OUTER JOIN forums ON (threads.forum_id = forums.id)",
	).Where(
		strings.Join(wheres, " AND "),
		placeholders...,
	).Order("comments.updated_at desc")
	query.Model(&Comment{}).Count(&pager.Total)

	// Execute the query.
	query = query.Offset(pager.GetOffset()).Limit(pager.PerPage).Find(&scan)
	return scan, query.Error
}

// ScanLatestForumCommentsPerThread returns a scan of Newest forum posts, deduplicated by thread.
// Each thread ID will only appear once in the result, paired with the newest comment in that
// thread.
func ScanLatestForumCommentsPerThread(wheres, comment_wheres []string, placeholders, comment_ph []interface{}, pager *Pagination) (NewestForumPostsScanner, error) {
	var (
		result    NewestForumPostsScanner
		threadIDs = []uint64{}

		// Query for ALL thread IDs (in forums the user can see).
		query = DB.Table(
			"threads",
		).Select(`
			DISTINCT ON (threads.id)
			threads.forum_id,
			threads.id AS thread_id,
			threads.updated_at AS updated_at
		`).Joins(
			"JOIN forums ON (threads.forum_id = forums.id)",
		).Where(
			strings.Join(wheres, " AND "),
			placeholders...,
		).Order(
			"threads.id",
		)
	)

	query = query.Find(&result)
	if query.Error != nil {
		return result, query.Error
	}
	pager.Total = int64(len(result))

	// Reorder the result by timestamp.
	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})

	// Subslice the result per the user's pagination setting.
	var (
		start = pager.GetOffset()
		stop  = start + pager.PerPage
	)
	if start > len(result) {
		return NewestForumPostsScanner{}, nil
	} else if stop > len(result) {
		stop = len(result)
	}
	result = result[start:stop]

	// Map the thread IDs to their result row.
	var threadMap = map[uint64]int{}
	for i, row := range result {
		threadIDs = append(threadIDs, *row.ThreadID)
		threadMap[*row.ThreadID] = i
	}

	// With these thread IDs, select the newest comments.
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
	).Where(
		strings.Join(comment_wheres, " AND "),
		comment_ph...,
	).Order(
		"updated_at desc",
	).Scan(&scan)
	if err.Error != nil {
		log.Error("Getting most recent post IDs: %s", err.Error)
		return result, err.Error
	}

	// Populate the comment IDs back in.
	for _, row := range scan {
		if idx, ok := threadMap[row.ThreadID]; ok {
			result[idx].CommentID = row.CommentID
		}
	}

	return result, query.Error
}
