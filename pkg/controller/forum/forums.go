package forum

import (
	"fmt"
	"net/http"
	"regexp"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Regular expressions
var (
	FragmentPattern = `[a-z0-9._-]{1,30}`
	FragmentRegexp  = regexp.MustCompile(
		fmt.Sprintf(`^(%s)$`, FragmentPattern),
	)
)

// Landing page for forums.
func Landing() http.HandlerFunc {
	tmpl := templates.Must("forum/index.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Get all the categorized index forums.
		// XXX: we get a large page size to get ALL official forums
		// This pager is hardcoded and doesn't parse from ?page= params.
		var indexPager = &models.Pagination{
			Page:    1,
			PerPage: config.PageSizeForums,
			Sort:    "title asc",
		}
		forums, err := models.PaginateForums(currentUser, config.ForumCategories, nil, false, indexPager)
		if err != nil {
			session.FlashError(w, r, "Couldn't paginate forums: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Bucket the forums into their categories for easy front-end.
		categorized := models.CategorizeForums(forums, config.ForumCategories)

		// Inject the "My List" Category if the user subscribes to forums.
		var pager = &models.Pagination{
			Page:    1,
			PerPage: config.PageSizeMyListForums,
			Sort:    "by_latest",
		}
		pager.ParsePage(r)
		if config.UserForumsEnabled {
			myList, err := models.PaginateForums(currentUser, nil, nil, true, pager)
			if err != nil {
				session.FlashError(w, r, "Couldn't get your followed forums: %s", err)
			} else if len(myList) > 0 {
				forums = append(forums, myList...)
				categorized = append([]*models.CategorizedForum{
					{
						Category: "My List",
						Forums:   myList,
					},
				}, categorized...)
			}
		}

		// Map statistics for these forums.
		forumMap := models.MapForumStatistics(forums)
		followMap := models.MapForumMemberships(currentUser, forums)

		var vars = map[string]interface{}{
			"Pager":        pager,
			"Categories":   categorized,
			"ForumMap":     forumMap,
			"FollowMap":    followMap,
			"FollowersMap": models.MapForumFollowers(forums),
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
