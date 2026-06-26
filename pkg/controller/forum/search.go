package forum

import (
	"net/http"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Search the forums.
func Search() http.HandlerFunc {
	tmpl := templates.Must("forum/search.html")

	// Whitelist for ordering options.
	var sortWhitelist = []string{
		"comments.created_at desc",
		"comments.created_at asc",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			searchTerm = r.FormValue("q")
			byUsername = r.FormValue("username")
			postType   = r.FormValue("type")
			inForum    = r.FormValue("in")
			inFragment = r.FormValue("fragment")
			withPhotos = r.FormValue("photos") == "true"
			categories = []string{}
			sort       = r.FormValue("sort")
			sortOK     bool
		)

		// Sort options.
		for _, v := range sortWhitelist {
			if sort == v {
				sortOK = true
				break
			}
		}
		if !sortOK {
			sort = sortWhitelist[0]
		}

		// All comments, or threads only?
		if postType != "threads" {
			postType = "all"
		}

		// In forums
		switch inForum {
		case "official":
			categories = config.ForumCategories
		case "community":
			categories = []string{""}
		}

		// Normalize the fragment URL.
		if inFragment != "" {
			// Remove `/f/` prefix if given, and query string.
			inFragment = strings.SplitN(strings.TrimPrefix(inFragment, "/f/"), "?", 2)[0]
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Filters: find the user ID.
		var filterUserID uint64
		if byUsername != "" {
			user, err := models.FindUsername(byUsername)
			if err != nil {
				session.FlashError(w, r, "Couldn't search posts by that username: no such username found.")
				templates.Redirect(w, r.URL.Path)
				return
			}
			filterUserID = user.ID
		}

		// Parse their search term.
		var (
			search  = models.ParseSearchString(searchTerm)
			filters = models.ForumSearchFilters{
				UserID:      filterUserID,
				ThreadsOnly: postType == "threads",
				Fragment:    inFragment,
				WithPhotos:  withPhotos,
			}
			pager = &models.Pagination{
				Page:    1,
				PerPage: config.PageSizeThreadList,
				Sort:    sort,
			}
		)
		pager.ParsePage(r)

		posts, err := models.SearchForum(currentUser, categories, search, filters, pager)
		if err != nil {
			session.FlashError(w, r, "Couldn't search the forums: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Map the originating threads to each comment.
		threadMap, err := models.MapForumCommentThreads(posts)
		if err != nil {
			log.Error("Couldn't map forum threads to comments: %s", err)
		}

		// Get any photo attachments for these comments.
		photos, err := models.MapCommentPhotos(posts)
		if err != nil {
			log.Error("Couldn't MapCommentPhotos: %s", err)
		}

		// Log the search terms for analytics.
		if searchTerm != "" {
			message := "Searched the forums by keyword: " + searchTerm
			models.LogEvent(currentUser, nil, models.ChangeLogAnalytics, "forums.search", 0, message)
		}

		var vars = map[string]interface{}{
			"CurrentForumTab": "search",
			"Pager":           pager,
			"Comments":        posts,
			"ThreadMap":       threadMap,
			"PhotoMap":        photos,

			"SearchTerm": searchTerm,
			"ByUsername": byUsername,
			"ByFragment": inFragment,
			"Type":       postType,
			"InForum":    inForum,
			"WithPhotos": withPhotos,
			"Sort":       sort,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
