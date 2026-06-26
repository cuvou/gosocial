package photo

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// SiteGallery controller (/photo/gallery) to view all members' public gallery pics.
func SiteGallery() http.HandlerFunc {
	tmpl := templates.Must("photo/gallery.html")

	// Whitelist for ordering options.
	var sortWhitelist = []string{
		"created_at desc",
		"created_at asc",
		"like_count desc",
		"comment_count desc",
		"views desc",
		"recently_commented",
		"random",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Query params.
		var (
			viewStyle = r.FormValue("view") // cards (default), full

			// Search filters.
			who              = r.FormValue("who")
			filterExplicit   = r.FormValue("explicit")
			filterVisibility = r.FormValue("visibility")
			filterGIF        = r.FormValue("gif")
			filterTagged     = r.FormValue("tagged") == "true"
			adminView        = r.FormValue("admin_view") == "true"
			sort             = r.FormValue("sort")
			sortOK           bool
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

		// Load the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: couldn't get CurrentUser")
		}

		// Defaults.
		if viewStyle != "full" {
			viewStyle = "cards"
		}
		if who == "" {
			// They didn't post a "Whose photos" filter, restore it from their last saved default.
			who = currentUser.GetProfileField("site_gallery_default")
		}
		if who != "friends" && who != "everybody" && who != "friends+private" && who != "likes" {
			// Default Who setting should be Everybody.
			who = "everybody"
		}

		// Store their "Whose photos" filter on their page to default it for next time.
		currentUser.SetProfileField("site_gallery_default", who)

		// Admin scope warning.
		if adminView && !currentUser.HasAdminScope(config.ScopePhotoModerator) {
			session.FlashError(w, r, "Missing admin scope: %s", config.ScopePhotoModerator)
		}

		// Get the page of photos.
		pager := &models.Pagination{
			Page:    1,
			PerPage: config.PageSizeSiteGallery,
			Sort:    sort,
		}
		pager.ParsePage(r)
		photos, _ := models.PaginateGalleryPhotos(currentUser, models.Gallery{
			Explicit:    filterExplicit,
			Visibility:  filterVisibility,
			GIF:         filterGIF,
			Tagged:      filterTagged,
			AdminView:   adminView,
			FriendsOnly: who == "friends",
			MyLikes:     who == "likes",
		}, pager)

		// Bulk load the users associated with these photos.
		var userIDs = []uint64{}
		for _, photo := range photos {
			userIDs = append(userIDs, photo.UserID)
		}
		userMap, err := models.MapUsers(currentUser, userIDs)
		if err != nil {
			session.FlashError(w, r, "Failed to MapUsers: %s", err)
		}

		// Get Likes information about these photos.
		var photoIDs = []uint64{}
		for _, p := range photos {
			photoIDs = append(photoIDs, p.ID)
		}
		likeMap := models.MapLikes(currentUser, "photos", photoIDs)
		commentMap := models.MapCommentCounts("photos", photoIDs)

		// Ping this user as having used the forums today.
		go func() {
			if err := models.LogDailyGalleryUser(currentUser); err != nil {
				log.Error("LogDailyGalleryUser(%s): error logging their usage statistic: %s", currentUser.Username, err)
			}
		}()

		var vars = map[string]interface{}{
			"IsSiteGallery": true,
			"Photos":        photos,
			"UserMap":       userMap,
			"LikeMap":       likeMap,
			"CommentMap":    commentMap,
			"Pager":         pager,
			"ViewStyle":     viewStyle,

			// Search filters
			"Sort":             sort,
			"FilterWho":        who,
			"FilterExplicit":   filterExplicit,
			"FilterVisibility": filterVisibility,
			"FilterGIF":        filterGIF,
			"FilterTagged":     filterTagged,
			"AdminView":        adminView,

			// Dummy PhotoInsights object for Quick Filters.
			"PhotoInsights": models.PhotoInsights{},
		}

		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
