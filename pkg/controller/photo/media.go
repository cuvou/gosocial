package photo

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/utility"
)

// MyMedia controller (/photo/media) to manage the current user's media (photos and comment photos).
func MyMedia() http.HandlerFunc {
	// Reuse the upload page but with an EditPhoto variable.
	tmpl := templates.Must("photo/media.html")

	var sortWhitelist = []string{
		"filesize desc",
		"updated_at desc",
		"updated_at asc",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			view   = utility.StringIn(r.FormValue("view"), []string{"gallery", "forum"}, "gallery")
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
			sort = sortWhitelist[0]
		}

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: could not get currentUser.")
			templates.Redirect(w, "/")
			return
		}

		// Which set of photos to paginate?
		var (
			galleryPhotos []*models.Photo
			commentPhotos []*models.CommentPhoto
			pager         = &models.Pagination{
				Page:    1,
				PerPage: config.PageSizeMyMedia,
				Sort:    sort,
			}
		)
		pager.ParsePage(r)

		switch view {
		case "gallery":
			if ps, err := models.PaginateUserPhotos(
				currentUser,
				currentUser.ID,
				models.UserGallery{
					Visibility: models.PhotoVisibilityAll,
				},
				pager,
			); err != nil {
				session.FlashError(w, r, "Error getting your gallery photos: %s", err)
			} else {
				galleryPhotos = ps
			}
		case "forum":
			if ps, err := models.PaginateUserCommentPhotos(
				currentUser.ID,
				pager,
			); err != nil {
				session.FlashError(w, r, "Error getting your comment photos: %s", err)
			} else {
				commentPhotos = ps
			}
		default:
			session.FlashError(w, r, "Unknown category of media.")
			templates.Redirect(w, r.URL.Path)
			return
		}

		// Get their media quota statistics.
		quota, err := models.GetUserMediaQuota(currentUser)
		if err != nil {
			session.FlashError(w, r, "Error when getting your media quota: %s", err)
		}

		_ = currentUser

		// Prepare the template.
		var vars = map[string]interface{}{
			"View":  view,
			"Sort":  sort,
			"Quota": quota,
			"Pager": pager,
		}

		// Photos to paginate
		if len(galleryPhotos) > 0 {
			vars["Photos"] = galleryPhotos
		} else if len(commentPhotos) > 0 {
			vars["Photos"] = commentPhotos
		}

		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
