package photo

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/utility"
)

// UserPhotos controller (/photo/u/:username) to view a user's gallery or manage if it's yourself.
func UserPhotos() http.HandlerFunc {
	tmpl := templates.Must("photo/gallery.html")

	// Whitelist for ordering options.
	var sortWhitelist = []string{
		"pinned desc nulls last, updated_at desc",
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
			username  = r.PathValue("username")
			viewStyle = r.FormValue("view")   // cards (default), full
			intent    = r.FormValue("intent") // e.g., profile_pic with instructions to set one

			// Search filters.
			filterExplicit   = r.FormValue("explicit")
			filterVisibility = r.FormValue("visibility")
			filterGIF        = r.FormValue("gif")
			sort             = utility.StringIn(r.FormValue("sort"), sortWhitelist, sortWhitelist[0])
		)

		// Defaults.
		if viewStyle != "full" {
			viewStyle = "cards"
		}

		// Find this user.
		user, err := models.FindUsername(username)
		if err != nil {
			templates.NotFoundPage(w, r)
			return
		}

		// Load the current user in case they are viewing their own page.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: couldn't get CurrentUser")
		}
		var (
			areFriends  = models.AreFriends(user.ID, currentUser.ID)
			isFollowing = models.IsFollowing(currentUser.ID, user.ID)
			isPrivate   = user.Visibility == models.UserVisibilityPrivate && !areFriends
			isOwnPhotos = currentUser.ID == user.ID
		)

		// Is either one blocking?
		// We check if the user SHOULD be seen by the viewer; so if the viewer is an admin and the user
		// has blocked us, we can still respect that block.
		if err := user.ShouldBeSeenBy(currentUser); err != nil {

			// But are we an admin?
			if currentUser.IsAdmin {
				// Still respect blocks if we are not Unblockable.
				if errors.Is(err, models.ErrUsersBlockingEachOther) && !currentUser.HasAdminScope(config.ScopeUnblockable) {

					// Log an admin report for visibility.
					fb := &models.Feedback{
						Intent:      "report",
						Subject:     "Blocking User Gallery Access Attempted",
						TableName:   "users",
						TableID:     currentUser.ID,
						AboutUserID: user.ID,
						Message: fmt.Sprintf(
							"The admin user **%s** (id:%d) has tried to view the Photo Gallery of [%s](/u/%s) (id:%d), but they were not allowed "+
								"because one of them is blocking the other.\n\nThis note is just recorded for visibility.",
							currentUser.Username, currentUser.ID,
							user.Username, user.Username, user.ID,
						),
					}
					if err := models.CreateFeedback(fb); err != nil {
						session.FlashError(w, r, "Couldn't create admin notification: %s", err)
					}

					session.FlashError(w, r, "This user's gallery is not available due to block lists (one of you blocks the other).")
					session.FlashError(w, r, "If you have concerns about their photo gallery, please contact another admin to have them take a look on your behalf.")
					templates.Redirect(w, "/u/"+user.Username)
					return
				}
			} else {
				templates.NotFoundPage(w, r)
				return
			}
		}

		// Is this user private and we're not friends?
		if isPrivate && !currentUser.IsAdmin && !isOwnPhotos {
			session.FlashError(w, r, "This user's profile page and photo gallery are private.")
			templates.Redirect(w, "/u/"+user.Username)
			return
		}

		// Has this user granted access to see their privates?
		var (
			isGrantee = models.IsPrivateUnlocked(user.ID, currentUser.ID) // THEY have granted US access
			isGranted = models.IsPrivateUnlocked(currentUser.ID, user.ID) // WE have granted THEM access
		)

		// What set of visibilities to query?
		visibility := []models.PhotoVisibility{models.PhotoPublic}
		if isOwnPhotos || isGrantee || currentUser.HasAdminScope(config.ScopePhotoModerator) {
			visibility = append(visibility, models.PhotoPrivate)
		}
		if isOwnPhotos || areFriends || currentUser.HasAdminScope(config.ScopePhotoModerator) {
			visibility = append(visibility, models.PhotoFriends)
		}

		// Record the full set of available visibilities, for the PhotoInsights later for quick filters.
		availableVisibility := visibility

		// If we are Filtering by Visibility, ensure the target visibility is accessible to us.
		if filterVisibility != "" {
			var isOK bool
			for _, allowed := range visibility {
				if allowed == models.PhotoVisibility(filterVisibility) {
					isOK = true
					break
				}
			}

			// If the filter is within the set we are allowed to see, update the set.
			if isOK {
				visibility = []models.PhotoVisibility{models.PhotoVisibility(filterVisibility)}
			} else {
				session.FlashError(w, r, "Could not filter pictures by that visibility setting: it is not available for you.")
				visibility = []models.PhotoVisibility{models.PhotoNotAvailable}
			}
		}

		// Explicit photo filter? The default ("") will defer to the user's Explicit opt-in.
		if filterExplicit == "" {
			// If the viewer does not opt-in to explicit AND is not looking at their own gallery,
			// then default the explicit filter to "do not show explicit"
			if !currentUser.Explicit && !isOwnPhotos {
				filterExplicit = "false"
			}
		}

		// Get the page of photos.
		pager := &models.Pagination{
			Page:    1,
			PerPage: config.PageSizeUserGallery,
			Sort:    sort,
		}
		pager.ParsePage(r)
		photos, err := models.PaginateUserPhotos(currentUser, user.ID, models.UserGallery{
			Explicit:   filterExplicit,
			GIF:        filterGIF,
			Visibility: visibility,
		}, pager)
		if err != nil {
			log.Error("PaginateUserPhotos(%s): %s", user.Username, err)
		}

		// Get the count of explicit photos if we are not viewing explicit photos.
		var explicitCount int64
		if filterExplicit == "false" {
			explicitCount, _ = models.CountExplicitPhotos(user.ID, visibility)
		}

		// Get Likes information about these photos.
		var photoIDs = []uint64{}
		for _, p := range photos {
			photoIDs = append(photoIDs, p.ID)
		}
		likeMap := models.MapLikes(currentUser, "photos", photoIDs)
		commentMap := models.MapCommentCounts("photos", photoIDs)

		// Can we see their default profile picture? If no: show a hint on the Gallery page that
		// their default pic isn't visible.
		var profilePictureHidden models.PhotoVisibility
		if ok, visibility := user.CanSeeProfilePicture(currentUser); !ok && visibility != models.PhotoPublic {
			profilePictureHidden = visibility
		}

		// Should the current user be able to share their private photos with the target?
		showPrivateUnlockPrompt, _ := models.ShouldShowPrivateUnlockPrompt(currentUser, user)

		// Get quick insights of this user's gallery (counts of visibilities, GIFs, etc.)
		photoInsights := models.GetPhotoInsights(currentUser, user, availableVisibility)

		// Profile tab counts.
		tabCounts, err := user.GetProfileTabCounts(currentUser)
		if err != nil {
			session.FlashError(w, r, "Error getting profile tab counts: %s", err)
		}

		// Set user relationship booleans.
		if err := models.SetUserRelationships(currentUser, []*models.User{user}); err != nil {
			session.FlashError(w, r, "Error setting user relationships: %s", err)
		}

		var vars = map[string]interface{}{
			"Intent":                         intent,
			"ActiveProfileTab":               "photos",
			"IsOwnPhotos":                    currentUser.ID == user.ID,
			"IsMyPrivateUnlockedFor":         isGranted, // have WE granted THIS USER to see our private pics?
			"AreWeGrantedPrivate":            isGrantee, // have THEY granted US private photo access.
			"ShowPrivateUnlockPrompt":        showPrivateUnlockPrompt,
			"AreFriends":                     areFriends,
			"IsFollowing":                    isFollowing,
			"ProfilePictureHiddenVisibility": profilePictureHidden,
			"User":                           user,
			"ProfileTheme":                   models.GetProfileTheme(user.ID),
			"Photos":                         photos,
			"ProfileTabCount":                tabCounts,
			"PublicPhotoCount":               models.CountPublicPhotos(user.ID),
			"Pager":                          pager,
			"LikeMap":                        likeMap,
			"CommentMap":                     commentMap,
			"ViewStyle":                      viewStyle,
			"ExplicitCount":                  explicitCount,
			"PhotoInsights":                  photoInsights,

			// Search filters
			"Sort":             sort,
			"FilterExplicit":   filterExplicit,
			"FilterVisibility": filterVisibility,
			"FilterGIF":        filterGIF,
		}

		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
