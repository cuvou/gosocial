package forum

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Newest posts across all of the (official) forums.
func Newest() http.HandlerFunc {
	tmpl := templates.Must("forum/newest.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Query parameters.
		var (
			allComments    = r.FormValue("all") == "true"
			whichForums    = r.FormValue("which")
			attachedPhotos = r.FormValue("photos") // show, hide
			categories     = []string{}
			subscribed     bool
		)

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Recall the user's default "Which forum:" answer if not selected.
		if whichForums == "" {
			whichForums = currentUser.GetProfileField("forum_newest_default")
			if whichForums == "" {
				whichForums = "official"
			}
		}

		// Does the user opt to hide attached photo previews?
		if attachedPhotos == "" {
			attachedPhotos = currentUser.GetProfileField("forum_newest_photos")
			if attachedPhotos == "" {
				attachedPhotos = "show"
			}
		}

		// Narrow down to which set of forums?
		switch whichForums {
		case "official":
			categories = config.ForumCategories
		case "community":
			categories = []string{""}
		case "followed":
			subscribed = true
		default:
			whichForums = "all"
		}

		// Store their "Which forums" filter to be their new default view.
		currentUser.SetProfileField("forum_newest_default", whichForums)
		currentUser.SetProfileField("forum_newest_photos", attachedPhotos)

		// Get all the categorized index forums.
		var pager = &models.Pagination{
			Page:    1,
			PerPage: config.PageSizeThreadList,
		}
		pager.ParsePage(r)

		posts, err := models.PaginateRecentPosts(currentUser, categories, subscribed, allComments, pager)
		if err != nil {
			session.FlashError(w, r, "Couldn't paginate forums: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Get any photo attachments for these comments.
		var comments = []*models.Comment{}
		for _, post := range posts {
			comments = append(comments, post.Comment, &post.Thread.Comment)
		}
		photos, err := models.MapCommentPhotos(comments)
		if err != nil {
			log.Error("Couldn't MapCommentPhotos: %s", err)
		}

		var vars = map[string]interface{}{
			"CurrentForumTab": "newest",
			"Pager":           pager,
			"RecentPosts":     posts,
			"PhotoMap":        photos,

			// Filter options.
			"WhichForums":    whichForums,
			"AllComments":    allComments,
			"AttachedPhotos": attachedPhotos,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
