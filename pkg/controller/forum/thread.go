package forum

import (
	"net/http"
	"slices"
	"strconv"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Thread view for the comment thread body of a forum post.
func Thread() http.HandlerFunc {

	handler := MakeThreadView()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the path parameters
		var idStr = r.PathValue("id")
		handler(idStr)(w, r)
	})
}

// MakeThreadView is a common UI handler for viewing a forum thread.
//
// It is shared between Forums (/forum/thread/{id}) and Places (/place/{fragment}/thread/{id}).
func MakeThreadView() func(idStr string) http.HandlerFunc {
	tmpl := templates.Must("forum/thread.html")
	return func(idStr string) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Parse the path parameters
			var (
				idStr  = r.PathValue("id")
				forum  *models.Forum
				thread *models.Thread

				// If neither the forum nor thread are explicit, show a hint to the user not to
				// share an explicit photo in their reply.
				explicitPhotoAllowed bool
			)

			if idStr == "" {
				templates.NotFoundPage(w, r)
				return
			} else {
				if threadID, err := strconv.Atoi(idStr); err != nil {
					session.FlashError(w, r, "Invalid thread ID in the address bar.")
					templates.Redirect(w, "/forum")
					return
				} else {
					// Load the thread.
					if found, err := models.GetThread(uint64(threadID)); err != nil {
						session.FlashError(w, r, "That thread does not exist.")
						templates.Redirect(w, "/forum")
						return
					} else {
						thread = found
						forum = &thread.Forum
					}
				}
			}

			// Get the current user.
			currentUser, err := session.CurrentUser(r)
			if err != nil {
				session.FlashError(w, r, "Couldn't get current user: %s", err)
				templates.Redirect(w, "/")
				return
			}

			// Is it a private forum?
			if !forum.CanBeSeenBy(currentUser) {
				templates.NotFoundPage(w, r)
				return
			}

			// Can we moderate this forum? (from a user-owned forum perspective,
			// e.g. can we delete threads and posts, not edit them)
			var canModerate = forum.CanBeModeratedBy(currentUser)

			// Would an explicit photo attachment be allowed?
			if forum.Explicit || thread.Explicit {
				explicitPhotoAllowed = true
			}

			// Ping the view count on this thread.
			if err := thread.View(currentUser.ID); err != nil {
				log.Error("Couldn't ping view count on thread %d: %s", thread.ID, err)
			}

			// Paginate the comments on this thread.
			var pager = &models.Pagination{
				Page:    1,
				PerPage: config.PageSizeThreadList,
				Sort:    "created_at asc",
			}
			pager.ParsePage(r)

			comments, err := models.PaginateComments(currentUser, "threads", thread.ID, canModerate, pager)
			if err != nil {
				session.FlashError(w, r, "Couldn't paginate comments: %s", err)
				templates.Redirect(w, "/")
				return
			}

			// Is the OP of the thread blocking the current user?
			// Normally all of their comments are hidden, including the first comment on the thread.
			// If we are on Page 1 and the Thread.CommentID is not on this page, insert a dummy first comment.
			if pager.Page == 1 {
				var firstCommentVisible bool
				for _, c := range comments {
					if c.ID == thread.CommentID {
						firstCommentVisible = true
						break
					}
				}

				if !firstCommentVisible {
					comments = slices.Insert(comments, 0, models.DummyCommentUnavailableOP)
				}
			}

			// Get the like map for these comments.
			commentIDs := []uint64{}
			for _, com := range comments {
				commentIDs = append(commentIDs, com.ID)
			}
			commentLikeMap := models.MapLikes(currentUser, "comments", commentIDs)

			// Get any photo attachments for these comments.
			photos, err := models.MapCommentPhotos(comments)
			if err != nil {
				log.Error("Couldn't MapCommentPhotos: %s", err)
			}

			// Is the current user subscribed to notifications on this thread?
			_, isSubscribed := models.IsSubscribed(currentUser, "threads", thread.ID)

			// Ping this user as having used the forums today.
			go func() {
				if err := models.LogDailyForumUser(currentUser); err != nil {
					log.Error("LogDailyForumUser(%s): error logging their usage statistic: %s", currentUser.Username, err)
				}
			}()

			// Is the user over their photo storage quota?
			var isOverQuota bool
			if forum.PermitPhotos {
				if ok, _, _ := models.IsOverQuota(currentUser); ok {
					isOverQuota = ok
				}
			}

			var vars = map[string]interface{}{
				"Forum":                forum,
				"Thread":               thread,
				"Comments":             comments,
				"LikeMap":              commentLikeMap,
				"PhotoMap":             photos,
				"Pager":                pager,
				"CanModerate":          canModerate,
				"IsSubscribed":         isSubscribed,
				"IsForumSubscribed":    models.IsForumSubscribed(currentUser.ID, forum.ID),
				"ExplicitPhotoAllowed": explicitPhotoAllowed,
				"IsOverQuota":          isOverQuota,
			}
			if err := tmpl.Execute(w, r, vars); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		})
	}
}
