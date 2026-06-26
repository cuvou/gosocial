package settings

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/photo"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/utility"
)

// Look & Feel settings (/settings/look).
func Look() http.HandlerFunc {
	tmpl := templates.Must("settings/look.html")
	var (
		reHexColor   = regexp.MustCompile(`^#[a-fA-F0-9]{6}$`)
		mustHexColor = func(v string) string {
			if !reHexColor.Match([]byte(v)) {
				return ""
			}
			return v
		}
	)

	// Common handler function to accept file uploads (hero background and wallpaper image).
	// inputName = the HTML file <input> name.
	// deleteName = the checkbox to delete the file.
	// currentValue = the stored value of the pre-existing file on the server (for deletions)
	// returns: filename, filesize, isDeleted, error
	var handleFileUpload = func(r *http.Request, inputName, deleteName string, currentValue string) (string, int64, bool, error) {
		// Delete the original filename if the checkbox is ticked (a new upload at the same time will be ignored).
		if isDelete := r.PostFormValue(deleteName) == "true"; isDelete {
			if currentValue != "" {
				if err := photo.Delete(currentValue); err != nil {
					return "", 0, false, fmt.Errorf("error deleting your background image: %s", err)
				}
			}
			return "", 0, true, nil
		} else if file, header, err := r.FormFile(inputName); err != nil {
			log.Error("Look & Feel: checking file input for %s: %s", inputName, err)
			return "", 0, false, nil
		} else {
			var buf bytes.Buffer
			io.Copy(&buf, file)
			filename, _, err := photo.UploadPhoto(photo.UploadConfig{
				Extension: filepath.Ext(header.Filename),
				Data:      buf.Bytes(),
			})

			// If there was a pre-existing file stored, delete it first.
			if currentValue != "" {
				if err := photo.Delete(currentValue); err != nil {
					return "", 0, false, fmt.Errorf("error deleting your old background image: %s", err)
				}
			}

			if err != nil {
				return "", 0, false, fmt.Errorf("error receiving your image upload: %s", err)
			} else {
				var filesize int64
				if stat, err := os.Stat(photo.DiskPath(filename)); err == nil {
					filesize = stat.Size()
				}
				return filename, filesize, false, nil
			}
		}
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Load the current user in case of updates.
		user, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get CurrentUser: %s", err)
			templates.Redirect(w, r.URL.Path)
			return
		}

		// Get their current ProfileTheme.
		pt := models.GetProfileTheme(user.ID)

		// Are we POSTing?
		if r.Method == http.MethodPost {

			// Resetting all styles?
			if r.PostFormValue("reset") == "true" {

				// Delete stored photos from disk.
				for _, filename := range []string{pt.HeroFilename, pt.WallpaperFilename} {
					if filename != "" {
						if err := photo.Delete(filename); err != nil {
							session.FlashError(w, r, "Error deleting background image %s: %s", filename, err)
						}
					}
				}

				if err := pt.Delete(); err != nil {
					session.FlashError(w, r, "Failed to save user to database: %s", err)
				} else {
					session.Flash(w, r, "Profile look & feel reset to defaults!")
				}

				templates.Redirect(w, r.URL.Path)
				return
			}

			// Uploading or deleting the hero image?
			if filename, filesize, isDelete, err := handleFileUpload(r, "hero_file", "delete_hero", pt.HeroFilename); err != nil {
				session.FlashError(w, r, "Error uploading the hero background image: %s", err)
			} else if isDelete || filename != "" {
				if isDelete {
					pt.HeroFilename = ""
					pt.HeroFilesize = 0
					session.Flash(w, r, "Your profile header background image has been deleted.")
				} else {
					pt.HeroFilename = filename
					pt.HeroFilesize = filesize
					session.Flash(w, r, "Received your header background image upload!")
				}
			}

			// Uploading or deleting the wallpaper image?
			if filename, filesize, isDelete, err := handleFileUpload(r, "wallpaper_file", "delete_wallpaper", pt.WallpaperFilename); err != nil {
				session.FlashError(w, r, "Error with the wallpaper background image: %s", err)
			} else if isDelete || filename != "" {
				if isDelete {
					pt.WallpaperFilename = ""
					pt.WallpaperFilesize = 0
					session.Flash(w, r, "Your profile wallpaper background image has been deleted.")
				} else {
					pt.WallpaperFilename = filename
					pt.WallpaperFilesize = filesize
					session.Flash(w, r, "Received your wallpaper image upload!")
				}
			}

			// Update their profile theme.
			pt.HeroColorStart = mustHexColor(r.PostFormValue("hero-color-start"))
			pt.HeroColorEnd = mustHexColor(r.PostFormValue("hero-color-end"))
			pt.HeroTextDark = r.PostFormValue("hero-text-dark") == "true"
			pt.CardTitleBG = mustHexColor(r.PostFormValue("card-title-bg"))
			pt.CardTitleFG = mustHexColor(r.PostFormValue("card-title-fg"))
			pt.CardLinkColor = mustHexColor(r.PostFormValue("card-link-color"))
			pt.CardLightness = r.PostFormValue("card-lightness")
			pt.CardCustomBG = r.PostFormValue("card-custom-bg")
			pt.CardCustomFG = r.PostFormValue("card-custom-fg")

			// Opacity.
			if opacity, err := strconv.ParseFloat(r.PostFormValue("hero_transparency"), 64); err == nil {
				pt.HeroTransparency = opacity
			}

			// Set their website theme preference too.
			user.SetProfileField("website-theme", r.PostFormValue("website-theme"))
			user.SetProfileField("website-theme-hue", utility.StringInOptGroup(
				r.PostFormValue("website-theme-hue"),
				config.WebsiteThemeHueChoices,
				"",
			))

			// Save profile theme.
			if err := pt.Save(); err != nil {
				session.FlashError(w, r, "Failed to save your profile theme to database: %s", err)
			}

			// Save user profile fields (website theme/hue).
			if err := user.Save(); err != nil {
				session.FlashError(w, r, "Failed to save user to database: %s", err)
			}

			session.Flash(w, r, "Profile look & feel updated!")
			templates.Redirect(w, r.URL.Path)
			return
		}

		vars := map[string]interface{}{
			"WebsiteThemeHueChoices": config.WebsiteThemeHueChoices,
			"ProfileTheme":           pt,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
