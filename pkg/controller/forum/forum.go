package forum

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Forum view for a specific board index (/f/{fragment}).
func Forum() http.HandlerFunc {

	handler := MakeBoardIndex()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse the path parameters
		var fragment = r.PathValue("fragment")

		handler(fragment)(w, r)
	})
}

// MakeBoardIndex is a common UI handler for viewing a forum board's index page.
//
// It is shared between Forums (/f/{fragment}) and Places (/place/{fragment}/discussion).
//
// For the Places use case, the Place is passed in and the UI shows the Place tab bar
// and header info. For the normal Forum use case, place is nil and the Forum UI is shown.
func MakeBoardIndex() func(fragment string) http.HandlerFunc {
	tmpl := templates.Must("forum/board_index.html")
	return func(fragment string) http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Locate the forum.
			var forum *models.Forum

			// Look up the forum by its fragment.
			if found, err := models.ForumByFragment(fragment); err != nil {
				templates.NotFoundPage(w, r)
				return
			} else {
				forum = found
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

			// Get the pinned threads.
			pinned, err := models.PinnedThreads(forum)
			if err != nil {
				session.FlashError(w, r, "Couldn't get pinned threads: %s", err)
				templates.Redirect(w, "/")
				return
			}

			// Get all the categorized index forums.
			// XXX: we get a large page size to get ALL official forums
			var pager = &models.Pagination{
				Page:    1,
				PerPage: config.PageSizeThreadList,
				Sort:    "threads.updated_at desc",
			}
			pager.ParsePage(r)

			threads, err := models.PaginateThreads(currentUser, forum, pager)
			if err != nil {
				session.FlashError(w, r, "Couldn't paginate threads: %s", err)
				templates.Redirect(w, "/")
				return
			}

			// Inject pinned threads on top of the first page.
			if pager.Page == 1 {
				threads = append(pinned, threads...)
			}

			// Map the statistics (replies, views) of these threads.
			threadMap := models.MapThreadStatistics(threads)

			// Load the forum's moderators.
			mods, err := forum.GetModerators()
			if err != nil {
				log.Error("Getting forum moderators: %s", err)
			}

			// Pull out the forum's Owner, null it out if they are blocked.
			var owner *models.User
			if forum.OwnerID > 0 && !models.IsBlocking(currentUser.ID, forum.OwnerID) {
				owner = &forum.Owner
			}

			var vars = map[string]any{
				"Forum":                forum,
				"ForumOwner":           owner,
				"ForumModerators":      mods,
				"ForumSubscriberCount": models.CountForumMemberships(forum),
				"IsForumSubscribed":    models.IsForumSubscribed(currentUser.ID, forum.ID),
				"Threads":              threads,
				"ThreadMap":            threadMap,
				"Pager":                pager,
			}
			if err := tmpl.Execute(w, r, vars); err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
		})
	}
}
