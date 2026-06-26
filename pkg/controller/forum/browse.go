package forum

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Explore all existing forums.
func Explore() http.HandlerFunc {
	// This page shares a template with the board index (Categories) page.
	tmpl := templates.Must("forum/index.html")

	// Whitelist for ordering options.
	var sortWhitelist = []string{
		"created_at desc",
		"created_at asc",
		"title asc",
		"title desc",

		// Special sort handlers.
		// See PaginateForums for expanded handlers for these.
		"by_followers",
		"by_latest",
		"by_threads",
		"by_posts",
		"by_users",
	}
	const defaultSort = "by_latest"

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			searchTerm = r.FormValue("q")
			search     = models.ParseSearchString(searchTerm)

			show       = r.FormValue("show")
			categories = []string{}

			subscribed = r.FormValue("show") == "followed"

			sort   = r.FormValue("sort")
			sortOK bool
		)

		// Sort options.
		for _, v := range sortWhitelist {
			if sort == v {
				sortOK = true
				break
			}
		}
		if !sortOK {
			sort = defaultSort
		}

		// Set of forum categories to filter for.
		switch show {
		case "official":
			categories = config.ForumCategories
		case "community":
			categories = []string{""}
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		var pager = &models.Pagination{
			Page:    1,
			PerPage: config.PageSizeBrowseForums,
			Sort:    sort,
		}
		pager.ParsePage(r)

		// Browse all forums (no category filter for official)
		forums, err := models.PaginateForums(currentUser, categories, search, subscribed, pager)
		if err != nil {
			session.FlashError(w, r, "Couldn't paginate forums: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Bucket the forums into their categories for easy front-end.
		categorized := models.CategorizeForums(forums, nil)

		// Map statistics for these forums.
		forumMap := models.MapForumStatistics(forums)
		followMap := models.MapForumMemberships(currentUser, forums)

		var vars = map[string]interface{}{
			"CurrentForumTab": "explore",
			"IsExploreTab":    true,
			"Pager":           pager,
			"Categories":      categorized,
			"ForumMap":        forumMap,
			"FollowMap":       followMap,
			"FollowersMap":    models.MapForumFollowers(forums),

			// Search filters
			"SearchTerm": searchTerm,
			"Show":       show,
			"Sort":       sort,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
