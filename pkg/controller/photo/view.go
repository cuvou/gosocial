package photo

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// View photo controller to see the comment thread.
func View() http.HandlerFunc {
	tmpl := templates.Must("photo/permalink.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Required query param: the photo ID.
		var photo *models.Photo
		if idStr := r.FormValue("id"); idStr == "" {
			session.FlashError(w, r, "Missing photo ID parameter.")
			templates.Redirect(w, "/")
			return
		} else {
			if idInt, err := strconv.Atoi(idStr); err != nil {
				session.FlashError(w, r, "Invalid ID parameter.")
				templates.Redirect(w, "/")
				return
			} else {
				if found, err := models.GetPhoto(uint64(idInt)); err != nil {
					templates.NotFoundPage(w, r)
					return
				} else {
					photo = found
				}
			}
		}

		// Load the current user in case they are viewing their own page.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: couldn't get CurrentUser")
		}

		// Find the photo's owner.
		user, err := models.GetUser(photo.UserID)
		if err != nil {
			templates.NotFoundPage(w, r)
			return
		}

		// For an admin viewer who is not Unblockable, still respect user blocklists.
		if currentUser.IsAdmin && !currentUser.HasAdminScope(config.ScopeUnblockable) {
			if models.IsBlocking(user.ID, currentUser.ID) {
				templates.NotFoundPage(w, r)
				return
			}
		}

		if ok, err := photo.CanBeSeenBy(currentUser); !ok {
			log.Error("Photo %d can't be seen by %s: %s", photo.ID, currentUser.Username, err)
			session.FlashError(w, r, "Photo Not Found")
			templates.Redirect(w, "/")
			return
		}

		// For admin users who can still see this photo (w/ moderator permission), check if
		// the photo is private or friends-only and should not have normally been visible; so
		// we can warn them before they leave a comment on it in case they'll spook its owner.
		var adminCommentWarning bool
		if currentUser.IsAdmin {
			if ok, _ := photo.ShouldBeSeenBy(currentUser); !ok {
				adminCommentWarning = true
			}
		}

		// Get Likes information about these photos.
		likeMap := models.MapLikes(currentUser, "photos", []uint64{photo.ID})
		commentMap := models.MapCommentCounts("photos", []uint64{photo.ID})

		// Get the summary of WHO liked this picture.
		likeExample, likeRemainder, err := models.WhoLikes(currentUser, "photos", photo.ID)
		if err != nil {
			log.Error("WhoLikes(photo %d): %s", photo.ID, err)
		}

		// Get the tagged users.
		tagged, err := models.GetTaggedUsers(currentUser, "photos", photo.ID)
		var imTagged bool
		if err != nil {
			log.Error("GetTaggedUsers(photo %d): %s", photo.ID, err)
		} else {
			for _, user := range tagged {
				if user.ID == currentUser.ID {
					imTagged = true
				}
			}
		}

		// Determine whether the photo owner wants us to be able to comment on it.
		var (
			privacySetting    = models.GetPrivacySetting(user.ID)
			commentPermission = privacySetting.PhotoComments
			canComment        = true
		)
		switch commentPermission {
		case "friends":
			if currentUser.ID != user.ID && !models.AreFriends(currentUser.ID, user.ID) {
				canComment = false
			}
		case "nobody":
			canComment = false
		}

		// Mark this photo as "Viewed" by the user.
		if err := photo.View(currentUser); err != nil {
			log.Error("Update photo(%d) views: %s", photo.ID, err)
		}

		var vars = map[string]interface{}{
			"IsOwnPhoto":  currentUser.ID == user.ID,
			"User":        user,
			"Photo":       photo,
			"LikeMap":     likeMap,
			"CommentMap":  commentMap,
			"TaggedUsers": tagged,
			"IAmTagged":   imTagged,

			"CanComment":          canComment,
			"CommentPermission":   commentPermission,
			"AdminCommentWarning": adminCommentWarning,

			// NextURL for comment thread.
			"CommentNextURL": fmt.Sprintf("%s?id=%d", r.URL.Path, photo.ID),

			// Details on who likes the photo.
			"LikeExample":   likeExample,
			"LikeRemainder": likeRemainder,
			"LikeTableName": "photos",
			"LikeTableID":   photo.ID,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
