package htmx

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/utility"
)

// UserStatistics for their profile page.
type UserStatistics struct {

	// General use statistics
	PhotoCount            int64 // Photos shared
	BlogCount             int64 // Blogs written
	FriendCount           int64 // Friends count
	ForumThreadCount      int64 // Forum threads started
	ForumReplyCount       int64 // Forum comments in total
	CommentPhotoCount     int64 // Photo count shared on forums
	PhotoCommentCount     int64 // Photo comments written
	CommentsReceivedCount int64 // Photo comments received
	LikesGivenCount       int64 // Total likes given
	LikesReceivedCount    int64 // Total likes received (all tables)

	// Our Relationship (from the perspective of the CurrentUser viewing the stats)
	OurMessages         int64 // DMs on the main website
	OurChatMessages     int64 // On the chat room
	OurCommentsReceived int64 // How many of my photos have you commented on?
	OurCommentsGiven    int64 // How many of their photos have I commented on?
	OurLikesReceived    int64
	OurLikesGiven       int64
}

// GetUserStatistics efficiently gathers all the counts and stats for the "Activity" card of profile pages.
func GetUserStatistics(currentUser, user *models.User) (UserStatistics, error) {
	var (
		result = UserStatistics{}

		areFriends        = models.AreFriends(user.ID, currentUser.ID)
		isPrivateUnlocked = models.IsPrivateUnlocked(user.ID, currentUser.ID)
		isSelf            = currentUser.ID == user.ID
		photoVisibility   = []models.PhotoVisibility{
			models.PhotoPublic,
		}

		// Name of BareRTC DMs channel_id.
		chatRoomChannelName = models.BareRTCDirectMessageChannelName([]string{
			currentUser.Username,
			user.Username,
		})
	)

	if areFriends || isSelf {
		photoVisibility = append(photoVisibility, models.PhotoFriends)
	}
	if isPrivateUnlocked || isSelf {
		photoVisibility = append(photoVisibility, models.PhotoPrivate)
	}

	// We will be doing a bunch of COUNT queries over many tables.
	var scanTables = []struct {
		Name  string
		SQL   string
		Param []any
	}{
		{
			// Count of photos on album.
			Name: "PhotoCount",
			SQL: `
				SELECT COUNT(*) AS metric_count
				FROM photos
				WHERE user_id = ?
				AND visibility IN ?
			`,
			Param: []any{user.ID, photoVisibility},
		},
		{
			// Count of friends.
			Name: "FriendCount",
			SQL: `
				SELECT COUNT(*) AS metric_count
				FROM friends
				WHERE target_user_id = ?
				AND approved IS TRUE
				AND EXISTS (
					SELECT 1
					FROM users
					WHERE id = friends.source_user_id
					AND users.status = 'active'
				)
			`,
			Param: []any{user.ID},
		},
		{
			// Count of forum threads started.
			Name: "ForumThreadCount",
			SQL: `
				SELECT COUNT(*) AS metric_count
				FROM comments
				JOIN threads ON (threads.comment_id = comments.id)
				WHERE comments.user_id = ?
			`,
			Param: []any{user.ID},
		},
		{
			// Count of all comments posted on forum.
			Name: "ForumReplyCount",
			SQL: `
				SELECT COUNT(*) AS metric_count
				FROM comments
				WHERE table_name = 'threads'
				AND comments.user_id = ?
			`,
			Param: []any{user.ID},
		},
		{
			// Count of photos shared on forum.
			Name: "CommentPhotoCount",
			SQL: `
				SELECT COUNT(*) AS metric_count
				FROM comment_photos
				WHERE comment_photos.user_id = ?
			`,
			Param: []any{user.ID},
		},
		{
			// Count of comments written on peoples' photos.
			Name: "PhotoCommentCount",
			SQL: `
				SELECT COUNT(*) AS metric_count
				FROM comments
				WHERE comments.table_name = 'photos'
				AND comments.user_id = ?
			`,
			Param: []any{user.ID},
		},
		{
			// Count of comments received on photos.
			Name: "CommentsReceivedCount",
			SQL: `
				SELECT COUNT(*) AS metric_count
				FROM comments
				LEFT OUTER JOIN photos ON (
					comments.table_name = 'photos'
					AND comments.table_id = photos.id
				)
				WHERE comments.table_name = 'photos'
				AND photos.user_id = ?
				AND photos.visibility IN ?
			`,
			Param: []any{user.ID, photoVisibility},
		},
		{
			// Count of likes given on anything.
			Name: "LikesGivenCount",
			SQL: `
				SELECT COUNT(*) AS metric_count
				FROM likes
				WHERE likes.user_id = ?
			`,
			Param: []any{user.ID},
		},
		{
			// Count of likes received on profile, photos, blogs and comments.
			Name: "LikesReceivedCount",
			SQL: `
				SELECT sum(c) AS metric_count FROM (
					SELECT count(*) AS c
					FROM likes
					WHERE likes.table_name = 'users' AND likes.table_id = ?

					UNION ALL

					SELECT count(*) AS c
					FROM likes
					JOIN photos ON (likes.table_name='photos' AND likes.table_id=photos.id)
					WHERE photos.user_id = ?
					AND photos.visibility IN ?

					UNION ALL

					SELECT count(*) AS c
					FROM likes
					JOIN comments ON (likes.table_name = 'comments' AND likes.table_id=comments.id)
					WHERE comments.user_id = ?
				) AS x
			`,
			Param: []any{
				user.ID,
				user.ID, photoVisibility,
				user.ID,
			},
		},

		// Our Relationship queries
		{
			// Count of main website DMs exchanged.
			Name: "OurMessages",
			SQL: `
				SELECT COUNT(*) AS metric_count
				FROM messages
				WHERE (
					source_user_id = ? AND target_user_id = ?
				) OR (
					target_user_id = ? AND source_user_id = ?
				)
			`,
			Param: []any{
				currentUser.ID, user.ID,
				currentUser.ID, user.ID,
			},
		},
		{
			// Count of chat room DMs exchanged.
			Name: "OurChatMessages",
			SQL: `
				SELECT COUNT(*) AS metric_count
				FROM direct_messages
				WHERE channel_id = ?
				AND deleted_at IS NULL
			`,
			Param: []any{
				chatRoomChannelName,
			},
		},
		{
			// How many of CurrentUser's photos were commented on by User?
			Name: "OurCommentsReceived",
			SQL: `
				SELECT COUNT(*) AS metric_count
				FROM comments
				JOIN photos ON (
					comments.table_name = 'photos'
					AND comments.table_id = photos.id
				)
				WHERE comments.user_id = ?
				AND comments.table_name='photos'
				AND photos.user_id = ?
			`,
			Param: []any{
				user.ID,
				currentUser.ID,
			},
		},
		{
			// How many of User's photos has CurrentUser commented on?
			Name: "OurCommentsGiven",
			SQL: `
				SELECT COUNT(*) AS metric_count
				FROM comments
				JOIN photos ON (
					comments.table_name = 'photos'
					AND comments.table_id = photos.id
				)
				WHERE comments.user_id = ?
				AND comments.table_name='photos'
				AND photos.user_id = ?
			`,
			Param: []any{
				currentUser.ID,
				user.ID,
			},
		},
		{
			// How many likes has User given to CurrentUser?
			Name: "OurLikesReceived",
			SQL: `
				SELECT sum(c) AS metric_count FROM (
					SELECT count(*) AS c
					FROM likes
					WHERE likes.table_name = 'users' AND likes.table_id = ?
					AND likes.user_id = ?

					UNION ALL

					SELECT count(*) AS c
					FROM likes
					JOIN photos ON (likes.table_name='photos' AND likes.table_id=photos.id)
					WHERE photos.user_id = ?
					AND likes.user_id = ?

					UNION ALL

					SELECT count(*) AS c
					FROM likes
					JOIN blogs ON (likes.table_name='blogs' AND likes.table_id=blogs.id)
					WHERE blogs.user_id = ?
					AND likes.user_id = ?

					UNION ALL

					SELECT count(*) AS c
					FROM likes
					JOIN comments ON (likes.table_name = 'comments' AND likes.table_id=comments.id)
					WHERE comments.user_id = ?
					AND likes.user_id = ?
				) AS x
			`,
			Param: []any{
				// 'Users' table: A is liked by B
				currentUser.ID, user.ID,

				// Photos table: A is photo owner, liked by B
				currentUser.ID, user.ID,

				// Blogs table: A is photo owner, liked by B
				currentUser.ID, user.ID,

				// Comments table: A liked by B
				currentUser.ID, user.ID,
			},
		},
		{
			// How many likes has CurrentUser given to User?
			Name: "OurLikesGiven",
			SQL: `
				SELECT sum(c) AS metric_count FROM (
					SELECT count(*) AS c
					FROM likes
					WHERE likes.table_name = 'users' AND likes.table_id = ?
					AND likes.user_id = ?

					UNION ALL

					SELECT count(*) AS c
					FROM likes
					JOIN photos ON (likes.table_name='photos' AND likes.table_id=photos.id)
					WHERE photos.user_id = ?
					AND likes.user_id = ?

					UNION ALL

					SELECT count(*) AS c
					FROM likes
					JOIN blogs ON (likes.table_name='blogs' AND likes.table_id=blogs.id)
					WHERE blogs.user_id = ?
					AND likes.user_id = ?

					UNION ALL

					SELECT count(*) AS c
					FROM likes
					JOIN comments ON (likes.table_name = 'comments' AND likes.table_id=comments.id)
					WHERE comments.user_id = ?
					AND likes.user_id = ?
				) AS x
			`,
			Param: []any{
				// 'Users' table: A is liked by B
				user.ID, currentUser.ID,

				// Photos table: A is photo owner, liked by B
				user.ID, currentUser.ID,

				// Blogs table: A is blog owner, liked by B
				user.ID, currentUser.ID,

				// Comments table: A liked by B
				user.ID, currentUser.ID,
			},
		},
	}

	// Assemble the query.
	var queryParts = []string{
		"WITH",
	}
	var placeholders = []any{}

	// Append the "WITH subquery_" chain.
	for i, table := range scanTables {
		subquery := fmt.Sprintf(`subquery_%s AS (%s)`, table.Name, table.SQL)
		if i > 0 {
			subquery = ", " + subquery
		}
		queryParts = append(queryParts, subquery)
		placeholders = append(placeholders, table.Param...)
	}

	// Append the UNION ALL selects from all the subqueries.
	for i, table := range scanTables {
		selectQuery := fmt.Sprintf(
			`SELECT
				'%s' AS metric_type,
				metric_count AS metric_count
			FROM subquery_%s`,
			table.Name, table.Name,
		)
		if i > 0 {
			queryParts = append(queryParts, "UNION ALL")
		}
		queryParts = append(queryParts, selectQuery)
	}

	// Execute the SQL.
	type record struct {
		MetricType  string
		MetricCount int64
	}
	var records []record
	res := models.DB.Raw(
		strings.Join(queryParts, " "),
		placeholders...,
	).Scan(&records)
	if res.Error != nil {
		return result, res.Error
	}

	// Ingest the records.
	for _, row := range records {
		switch row.MetricType {
		case "PhotoCount":
			result.PhotoCount = row.MetricCount
		case "FriendCount":
			result.FriendCount = row.MetricCount
		case "ForumThreadCount":
			result.ForumThreadCount = row.MetricCount
		case "ForumReplyCount":
			result.ForumReplyCount = row.MetricCount
		case "CommentPhotoCount":
			result.CommentPhotoCount = row.MetricCount
		case "PhotoCommentCount":
			result.PhotoCommentCount = row.MetricCount
		case "CommentsReceivedCount":
			result.CommentsReceivedCount = row.MetricCount
		case "LikesGivenCount":
			result.LikesGivenCount = row.MetricCount
		case "LikesReceivedCount":
			result.LikesReceivedCount = row.MetricCount
		case "OurMessages":
			result.OurMessages = row.MetricCount
		case "OurChatMessages":
			result.OurChatMessages = row.MetricCount
		case "OurCommentsReceived":
			result.OurCommentsReceived = row.MetricCount
		case "OurCommentsGiven":
			result.OurCommentsGiven = row.MetricCount
		case "OurLikesReceived":
			result.OurLikesReceived = row.MetricCount
		case "OurLikesGiven":
			result.OurLikesGiven = row.MetricCount
		default:
			return result, fmt.Errorf("unknown statistic: %d", row.MetricCount)
		}
	}

	return result, nil
}

// Statistics and social activity on the user's profile page.
func UserProfileActivityCard() http.HandlerFunc {
	tmpl := templates.MustLoadCustom("partials/htmx/profile_activity.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			username = r.FormValue("username")
		)

		if username == "" {
			templates.NotFoundPage(w, r)
			return
		}

		// Debug: use ?delay=true to force a slower response.
		if r.FormValue("delay") != "" {
			time.Sleep(1 * time.Second)
		}

		// Find this user.
		user, err := models.FindUsername(username)
		if err != nil {
			templates.NotFoundPage(w, r)
			return
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "You must be signed in to view this page.")
			templates.Redirect(w, "/login?next=/u/"+url.QueryEscape(r.URL.String()))
			return
		}

		// Inject relationship booleans for profile picture display.
		models.SetUserRelationships(currentUser, []*models.User{user})

		// Give a Not Found page if we can not see this user.
		if err := user.CanBeSeenBy(currentUser); err != nil {
			log.Error("%s can not be seen by viewer %s: %s", user.Username, currentUser.Username, err)
			templates.NotFoundPage(w, r)
			return
		}

		// New stats
		stats, err := GetUserStatistics(currentUser, user)
		if err != nil {
			log.Error("ProfileActivity: GetUserStatistics: %s", err)
		}

		// If we are friends, get the Friend request so we can show for how long.
		var (
			friendsSince string
			friendsTime  time.Time
		)
		if m, err := models.MapFriendRequests(currentUser, []*models.User{user}, "friends"); err == nil {
			if friend, ok := m[user.ID]; ok {
				friendsSince = utility.FormatDurationCoarse(time.Since(friend.UpdatedAt))
				friendsTime = friend.UpdatedAt
			}
		}

		vars := map[string]interface{}{
			"User":              user,
			"UserStats":         stats,
			"MutualFriendCount": models.CountMutualFriends(currentUser, user),
			"FriendsSince":      friendsSince,
			"FriendsTime":       friendsTime,
		}

		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
