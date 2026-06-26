package photo

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/cuvou/gosocial/pkg/chat"
	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/notification"
	"github.com/cuvou/gosocial/pkg/photo"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/spam"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Upload photos controller.
func Upload() http.HandlerFunc {
	tmpl := templates.Must("photo/upload.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var vars = map[string]interface{}{
			"Intent":    r.FormValue("intent"),
			"NeedsCrop": false,
		}

		// Query string parameters: what is the intent of this photo upload?
		// - If profile picture, the user will crop their image before posting it.
		// - If regular photo, user simply picks a picture and doesn't need to crop it.
		if vars["Intent"] == "profile_pic" {
			vars["NeedsCrop"] = true
		}

		user, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: couldn't get CurrentUser")
		}

		// Check whether the user exceeds their photo quota.
		overQuota, _, err := models.IsOverQuota(user)
		if err != nil {
			log.Error("Error checking if user IsOverQuota: %s", err)
		}
		vars["IsOverQuota"] = overQuota

		// How many photos do they already have?
		var photoCount = models.CountPhotos(user.ID)
		vars["PhotoCount"] = photoCount

		// Is the user throttled from sharing a Site Gallery photo too frequently?
		vars["SiteGalleryThrottled"] = models.IsSiteGalleryThrottled(user, nil)
		vars["SiteGalleryThrottleLimit"] = config.SiteGalleryRateLimitMax

		// If they do not have a profile picture currently set (and are not uploading one now),
		// the front-end should point this out to them.
		if (user.ProfilePhotoID == nil || *user.ProfilePhotoID == 0) && vars["Intent"] != "profile_pic" {
			// If they have no photo at all, make the default intent to upload one.
			if photoCount == 0 {
				templates.Redirect(w, r.URL.Path+"?intent=profile_pic")
				return
			}
			vars["NoProfilePicture"] = true
		}

		// Are they POSTing?
		if r.Method == http.MethodPost {
			var (
				caption    = strings.TrimSpace(r.PostFormValue("caption"))
				altText    = strings.TrimSpace(r.PostFormValue("alt_text"))
				isExplicit = r.PostFormValue("explicit") == "true"
				visibility = r.PostFormValue("visibility")
				isGallery  = r.PostFormValue("gallery") == "true"
				isPinned   = r.PostFormValue("pinned") == "true"
				cropCoords = r.PostFormValue("crop")
				confirm1   = r.PostFormValue("confirm1") == "true"
				confirm2   = r.PostFormValue("confirm2") == "true"
			)

			// Enforce that they can not override the Site Gallery throttle.
			if vars["SiteGalleryThrottled"].(bool) && isGallery {
				isGallery = false
			}

			if len(altText) > config.AltTextMaxLength {
				altText = altText[:config.AltTextMaxLength]
			}

			// Are they at quota already?
			if overQuota {
				session.FlashError(w, r, "You have too many photos to upload a new one. Please delete a photo to make room for a new one.")
				templates.Redirect(w, "/u/"+user.Username+"/photos")
				return
			}

			// They checked both boxes. The browser shouldn't allow them to
			// post but validate it here anyway...
			if !confirm1 || !confirm2 {
				session.FlashError(w, r, "You must agree to the terms to upload this picture.")
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Parse and validate crop coordinates.
			var crop []int
			if vars["NeedsCrop"] == true {
				crop = photo.ParseCropCoords(cropCoords)
			}

			// Get their file upload.
			file, header, err := r.FormFile("file")
			if err != nil {
				session.FlashError(w, r, "Error receiving your file: %s", err)
				templates.Redirect(w, r.URL.Path)
				return
			}

			// GIF can not be uploaded for a profile picture.
			if filepath.Ext(header.Filename) == ".gif" && vars["Intent"] == "profile_pic" {
				session.FlashError(w, r, "GIF images are not acceptable for your profile picture.")
				templates.Redirect(w, r.URL.Path)
				return
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

			// Read the file contents.
			log.Debug("Receiving uploaded file (%d bytes): %s", header.Size, header.Filename)
			var buf bytes.Buffer
			io.Copy(&buf, file)

			filename, cropFilename, err := photo.UploadPhoto(photo.UploadConfig{
				User:      user,
				Extension: filepath.Ext(header.Filename),
				Data:      buf.Bytes(),
				Crop:      crop,
			})
			if err != nil {
				session.FlashError(w, r, "Error in UploadPhoto: %s", err)
				templates.Redirect(w, r.URL.Path)
				return
			}

			// Configuration for the DB entry.
			ptmpl := models.Photo{
				UserID:          user.ID,
				Filename:        filename,
				CroppedFilename: cropFilename,
				Caption:         caption,
				AltText:         altText,
				Visibility:      models.PhotoVisibility(visibility),
				Gallery:         isGallery,
				Pinned:          isPinned,
				Explicit:        isExplicit,
			}

			// Get the filesize.
			if stat, err := os.Stat(photo.DiskPath(filename)); err == nil {
				ptmpl.Filesize = stat.Size()
			}

			// Create it in DB!
			p, err := models.CreatePhoto(ptmpl)
			if err != nil {
				session.FlashError(w, r, "Couldn't create Photo in DB: %s", err)
			} else {
				log.Info("New photo! %+v", p)
			}

			// Test the uploaded file for A.I. metadata tags.
			if err := photo.TestAndReportAIPhoto(user, "Gallery Photo", header.Filename, file, "photos", p.ID); err != nil {
				log.Error("Error testing a photo for AI: %s", err)
			}

			// Are we uploading a profile pic? If so, set the user's pic now.
			if vars["Intent"] == "profile_pic" && cropFilename != "" {
				log.Info("User %s is setting their profile picture", user.Username)
				user.ProfilePhoto = *p
				user.Save()
			}

			// ChangeLog entry.
			models.LogCreated(user, "photos", p.ID, fmt.Sprintf(
				"Uploaded a new photo.\n\n"+
					"* Caption: %s\n"+
					"* Visibility: %s\n"+
					"* Gallery: %v\n"+
					"* Explicit: %v",
				p.Caption,
				p.Visibility,
				p.Gallery,
				p.Explicit,
			))

			// Notify all of our friends that we posted a new picture.
			go notification.NotifyFriendsNewPhoto(user, p)

			// If the user is currently on chat, push their updated JWT token
			// so we refresh their profile picture and/or Shy Account status.
			go func() {
				if err := chat.AmendJWTToken(r, user.ID); err != nil {
					log.Error("AmendJWTToken: Couldn't send amended JWT token for %s to chat room: %w", user.Username, err)
				}
			}()

			session.Flash(w, r, "Your photo has been uploaded successfully.")
			templates.Redirect(w, "/u/"+user.Username+"/photos")
			return
		}

		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
