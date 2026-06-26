package photo

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/cuvou/gosocial/pkg/chat"
	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	pphoto "github.com/cuvou/gosocial/pkg/photo"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/spam"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Edit controller (/photo/edit?id=N) to change properties about your picture.
func Edit() http.HandlerFunc {
	// Reuse the upload page but with an EditPhoto variable.
	tmpl := templates.Must("photo/upload.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Query params.
		photoID, err := strconv.Atoi(r.FormValue("id"))
		if err != nil {
			session.FlashError(w, r, "Photo 'id' parameter required.")
			templates.Redirect(w, "/")
			return
		}

		// Find this photo by ID.
		photo, err := models.GetPhoto(uint64(photoID))
		if err != nil {
			templates.NotFoundPage(w, r)
			return
		}

		// Load the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: couldn't get CurrentUser")
			templates.Redirect(w, "/")
			return
		}

		// In case an admin is editing this photo: remember the HTTP request current user,
		// before the currentUser may be set to the photo's owner below.
		var requestUser = currentUser

		// Do we have permission for this photo?
		if photo.UserID != currentUser.ID {
			if !currentUser.HasAdminScope(config.ScopePhotoModerator) {
				templates.ForbiddenPage(w, r)
				return
			}

			// Find the owner of this photo and assume currentUser is them for the remainder
			// of this controller.
			if user, err := models.GetUser(photo.UserID); err != nil {
				session.FlashError(w, r, "Couldn't get the owner User for this photo!")
				templates.Redirect(w, "/")
				return
			} else {
				currentUser = user
			}
		}

		// Is the user throttled for Site Gallery photo uploads?
		var SiteGalleryThrottled = models.IsSiteGalleryThrottled(currentUser, photo)

		// Are we saving the changes?
		if r.Method == http.MethodPost {
			var (
				caption    = strings.TrimSpace(r.FormValue("caption"))
				altText    = strings.TrimSpace(r.FormValue("alt_text"))
				isExplicit = r.FormValue("explicit") == "true"
				isGallery  = r.FormValue("gallery") == "true"
				isPinned   = r.FormValue("pinned") == "true"
				visibility = models.PhotoVisibility(r.FormValue("visibility"))
				rotation   = r.FormValue("rotation")

				// Profile pic fields
				setProfilePic = r.FormValue("intent") == "profile-pic"
				crop          = pphoto.ParseCropCoords(r.FormValue("crop"))

				// Are we GOING private?
				goingPrivate = visibility == models.PhotoPrivate && visibility != photo.Visibility
			)

			if len(altText) > config.AltTextMaxLength {
				altText = altText[:config.AltTextMaxLength]
			}

			// Respect the Site Gallery throttle in case the user is messing around.
			if SiteGalleryThrottled {
				isGallery = false
			}

			// Don't allow Telegram spam in captions.
			if err := spam.DetectContactSpam(caption); err != nil {
				session.FlashError(w, r, "%s", err.Error())
				caption = ""
			}
			if err := spam.DetectContactSpam(altText); err != nil {
				session.FlashError(w, r, "%s", err.Error())
				altText = ""
			}

			// Diff for the ChangeLog.
			diffs := []models.FieldDiff{
				models.NewFieldDiff("Caption", photo.Caption, caption),
				models.NewFieldDiff("Explicit", photo.Explicit, isExplicit),
				models.NewFieldDiff("Gallery", photo.Gallery, isGallery),
				models.NewFieldDiff("Pinned", photo.Pinned, isPinned),
				models.NewFieldDiff("Visibility", photo.Visibility, visibility),
			}

			photo.Caption = caption
			photo.AltText = altText
			photo.Explicit = isExplicit
			photo.Gallery = isGallery
			photo.Pinned = isPinned
			photo.Visibility = visibility

			// Can not use a GIF as profile pic.
			if setProfilePic && filepath.Ext(photo.Filename) == ".gif" {
				session.FlashError(w, r, "You can not use a GIF as your profile picture.")
				templates.Redirect(w, "/")
				return
			}

			// Are we applying a rotation to the image?
			if rotation != "" {
				deg, err := strconv.Atoi(rotation)
				if err != nil {
					session.FlashError(w, r, "Invalid rotation setting.")
					templates.Redirect(w, "/")
					return
				}

				if err := pphoto.Rotate(photo, deg); err != nil {
					session.FlashError(w, r, "Error rotating your photo: %s", err)
					templates.Redirect(w, "/")
					return
				} else {
					session.Flash(w, r, "Your photo was rotated by %d degrees.", deg)
				}
			}

			// Are we cropping ourselves a new profile pic?
			if setProfilePic && crop != nil && len(crop) >= 4 {
				cropFilename, err := pphoto.ReCrop(photo.Filename, crop[0], crop[1], crop[2], crop[3])
				if err != nil {
					session.FlashError(w, r, "Couldn't re-crop for profile picture: %s", err)
				} else {
					// If there was an old profile pic, remove it from disk.
					if photo.CroppedFilename != "" {
						pphoto.Delete(photo.CroppedFilename)
					}
					photo.CroppedFilename = cropFilename
				}
			} else {
				setProfilePic = false
			}

			if err := photo.Save(); err != nil {
				session.FlashError(w, r, "Couldn't save photo: %s", err)
			}

			// Set their profile pic to this one.
			if setProfilePic {
				currentUser.ProfilePhoto = *photo
				log.Error("Set user ProfilePhotoID=%d", photo.ID)
				if err := currentUser.Save(); err != nil {
					session.FlashError(w, r, "Couldn't save user: %s", err)
				}
			}

			// Flash success.
			session.Flash(w, r, "Photo settings updated!")

			// Log the change.
			models.LogUpdated(currentUser, requestUser, "photos", photo.ID, "Updated the photo's settings.", diffs)

			// If this picture has moved to Private, revoke any notification we gave about it before.
			if goingPrivate {
				log.Info("The picture is GOING PRIVATE (to %s), revoke any notifications about it", photo.Visibility)
				models.RemoveNotification("photos", photo.ID)
			}

			// If the user is currently on chat, push their updated JWT token
			// so we refresh their profile picture and/or Shy Account status.
			go func() {
				if err := chat.AmendJWTToken(r, currentUser.ID); err != nil {
					log.Error("AmendJWTToken: Couldn't send amended JWT token for %s to chat room: %w", currentUser.Username, err)
				}
			}()

			// Return the user to their gallery.
			templates.Redirect(w, "/u/"+currentUser.Username+"/photos")
			return
		}

		var vars = map[string]interface{}{
			"EditPhoto":                photo,
			"SiteGalleryThrottled":     SiteGalleryThrottled,
			"SiteGalleryThrottleLimit": config.SiteGalleryRateLimitMax,

			// Available admin labels enum.
			"RequestUser": requestUser,
		}

		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}

// Delete controller (/photo/Delete?id=N) to change properties about your picture.
//
// DEPRECATED: send them to the batch-edit endpoint.
func Delete() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		templates.Redirect(w, fmt.Sprintf("/photo/batch-edit?intent=delete&id=%s", r.FormValue("id")))
	})
}
