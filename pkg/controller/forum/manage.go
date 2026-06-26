package forum

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Manage page for forums -- admin only for now but may open up later.
func Manage() http.HandlerFunc {
	tmpl := templates.Must("forum/admin.html")

	// Whitelist for ordering options.
	var sortWhitelist = []string{
		"updated_at desc",
		"created_at desc",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			searchTerm = r.FormValue("q")
			show       = r.FormValue("show")
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

		// Show options.
		if show == "official" {
			categories = config.ForumCategories
		} else if show == "community" {
			categories = []string{""}
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Parse their search term.
		var search = models.ParseSearchString(searchTerm)

		// Get forums the user owns or can manage.
		var pager = &models.Pagination{
			Page:    1,
			PerPage: config.PageSizeForumAdmin,
			Sort:    sort,
		}
		pager.ParsePage(r)

		forums, err := models.PaginateOwnedForums(
			currentUser.ID,
			currentUser.HasAdminScope(config.ScopeForumAdmin),
			categories,
			search,
			pager,
		)
		if err != nil {
			session.FlashError(w, r, "Couldn't paginate owned forums: %s", err)
			templates.Redirect(w, "/")
			return
		}

		var vars = map[string]interface{}{
			"Pager":  pager,
			"Forums": forums,

			// Search filters.
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
