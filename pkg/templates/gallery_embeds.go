package templates

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
)

// Support functions for Markdown posts to reference gallery images and embed them inline.
var (
	MarkdownNakedLinkRegexp = regexp.MustCompile(`<p>(?:<a .+?>)?(https?://.+?)(?:</a>)?</p>`)
)

// CollectGalleryLinks searches a message for permalink URLs to site gallery photos, returning the photo IDs.
//
// This searches for image patterns ANYWHERE, and is intended for the Footer attachments, e.g. on forum threads.
func CollectGalleryLinks(message string) []uint64 {
	var (
		result   []uint64
		distinct = map[int]interface{}{}
	)

	m := config.PhotoPermalinkRegexp.FindAllStringSubmatch(message, -1)
	for _, match := range m {
		uri, err := url.Parse(match[0])
		if err != nil {
			log.Error("markdown.CollectGalleryLinks: URL %s didn't parse: %s", match, err)
			continue
		}

		query := uri.Query()
		if idStr, ok := query["id"]; ok {
			if i, err := strconv.Atoi(idStr[0]); err == nil {
				if _, ok := distinct[i]; ok {
					continue
				}
				distinct[i] = nil
				result = append(result, uint64(i))
			}
		}
	}

	return result
}

// CollectGalleryNakedLinks searches a message for "naked" permalink URLs to site gallery photos, returning the photo IDs.
//
// This is intended for inline image embedding, and the links must be on their own paragraph (with a blank line before/after,
// including the start/end of the message) for them to match.
//
// Note: the message must be post-rendered HTML.
//
// Returns a map of the original Markdown paragraph (to find/replace) to the photo ID
// parsed from its link.
func CollectGalleryNakedLinks(message template.HTML) map[string]uint64 {
	var (
		result   = map[string]uint64{}
		distinct = map[int]interface{}{}

		nakedLinks = MarkdownNakedLinkRegexp.FindAllStringSubmatch(string(message), -1)
	)

	for _, m := range nakedLinks {
		var (
			paragraph = m[0]
			link      = m[1]
		)

		// See if it is a photo gallery permalink.
		m := config.PhotoPermalinkRegexp.FindAllStringSubmatch(link, -1)
		for _, match := range m {
			uri, err := url.Parse(match[0])
			if err != nil {
				log.Error("markdown.CollectGalleryNakedLinks: URL %s didn't parse: %s", match, err)
				continue
			}

			query := uri.Query()
			if idStr, ok := query["id"]; ok {
				if i, err := strconv.Atoi(idStr[0]); err == nil {
					if _, ok := distinct[i]; ok {
						continue
					}
					distinct[i] = nil

					// Map the raw Markdown paragraph to this photo ID.
					result[paragraph] = uint64(i)
				}
			}
		}
	}

	return result
}

// ToMarkdownWithGalleryLinksInline will expand a gallery permalink with its image directly in-line (e.g. for profile pages).
//
// This allows users to reference their gallery photo and display them (just the image!) anywhere they want.
//
// Requirement: the link must be "naked" on its own paragraph without any surrounding text.
func ToMarkdownWithGalleryLinksInline(r *http.Request, currentUser *models.User, message string) template.HTML {
	var (
		rendered     = ToMarkdown(message)
		inlinePhotos = CollectGalleryNakedLinks(rendered)
		photoIDs     = []uint64{}
	)

	for _, photoID := range inlinePhotos {
		photoIDs = append(photoIDs, photoID)
	}

	// Get these photos and render the template.
	if photos, userMap, err := GetFilteredEmbeddedPhotos(currentUser, photoIDs); err == nil {
		tmpl, err := LoadCustom("partials/funcs/markdown_gallery.html")
		if err != nil {
			return template.HTML(fmt.Sprintf("[load template error: %s]", err))
		}

		// Map these photos for lookup by ID.
		var photoMap = map[uint64]*models.Photo{}
		for _, photo := range photos {
			photoMap[photo.ID] = photo
		}

		// Render the template for each photo individually, then replace it back
		// inline where it was originally referenced.
		for paragraph, photoID := range inlinePhotos {
			if photo, ok := photoMap[photoID]; !ok {
				continue
			} else {
				var (
					buf  = bytes.NewBuffer([]byte{})
					vars = map[string]interface{}{
						"Photos":   []*models.Photo{photo},
						"UserMap":  userMap,
						"IsInline": true,
					}
				)
				if err := tmpl.Execute(buf, r, vars); err != nil {
					return template.HTML(fmt.Sprintf("[template error: %s]", err))
				}

				// Replace the original Markdown paragraph.
				rendered = template.HTML(strings.Replace(string(rendered), paragraph, buf.String(), 1))
			}
		}

		return rendered
	}

	return rendered
}

// LazyLoadGalleryLinksFooter will parse gallery permalinks from a message and render
// an HTMLX lazy load to fetch and populate the embedded images in the footer of the message.
func LazyLoadGalleryLinksFooter(message string) template.HTML {
	var photoIDs = CollectGalleryLinks(message)
	if len(photoIDs) == 0 {
		return template.HTML("")
	}

	var strIDs = []string{}
	for _, id := range photoIDs {
		strIDs = append(strIDs, fmt.Sprintf("%d", id))
	}

	var (
		csv    = strings.Join(strIDs, ",")
		plural = "s"
	)
	if len(photoIDs) == 1 {
		plural = ""
	}

	return template.HTML(fmt.Sprintf(`
		<div hx-get="/htmx/photo/footer?ids=%s" hx-trigger="revealed" class="my-4">
			<i class="fa fa-spinner fa-spin"></i> Loading %d mentioned photo%s...
		</div>
	`, csv, len(photoIDs), plural))
}

// AttachMarkdownGalleryLinksFooter will parse gallery permalinks from a message and render
// an HTML footer (e.g. for forum posts) that embeds the images.
//
// Notice: this function can be slow to call for a large amount of photos at once, e.g. on a forum thread
// with many comments. It is better to use the LazyLoadGalleryLinksFooter which uses HTMX to load each
// batch of photos on demand. This func is not exposed to TemplateFuncs and is used only on the Markdown
// preview endpoint (because it is already an ajax request and HTMX doesn't fire well coming back from there).
func AttachMarkdownGalleryLinksFooter(r *http.Request, currentUser *models.User, message string) template.HTML {
	var (
		photoIDs = CollectGalleryLinks(message)
	)

	// Get these photos and render the template.
	if photos, userMap, err := GetFilteredEmbeddedPhotos(currentUser, photoIDs); err == nil {
		// Using the HTMX partial.
		tmpl, err := LoadCustom("partials/htmx/gallery_embeds.html")
		if err != nil {
			return template.HTML(fmt.Sprintf("[load template error: %s]", err))
		}

		var (
			buf  = bytes.NewBuffer([]byte{})
			vars = map[string]interface{}{
				"Photos":  photos,
				"UserMap": userMap,
			}
		)
		if err := tmpl.Execute(buf, r, vars); err != nil {
			return template.HTML(fmt.Sprintf("[template error: %s]", err))
		}

		return template.HTML(buf.String())
	}

	return template.HTML("")
}

func GetFilteredEmbeddedPhotos(currentUser *models.User, photoIDs []uint64) ([]*models.Photo, models.UserMap, error) {
	if len(photoIDs) == 0 {
		return nil, nil, nil
	}

	photoMap, err := models.MapPhotos(photoIDs)
	if err != nil {
		return nil, nil, fmt.Errorf("AttachMarkdownGalleryLinksFooter: getting photos: %w", err)
	}

	// Narrow it down to photos the current user can see.
	var (
		canSee  = []*models.Photo{}
		userIDs = []uint64{}
	)
	for _, photoID := range photoIDs {
		if photo, ok := photoMap[photoID]; !ok {
			continue
		} else {
			if ok, _ := photo.CanBeSeenBy(currentUser); !ok {
				continue
			}

			// Skip if the viewer doesn't want explicit photos.
			if photo.Explicit && !currentUser.Explicit {
				continue
			}

			canSee = append(canSee, photo)
			userIDs = append(userIDs, photo.UserID)
		}
	}

	// Any photos we can see?
	if len(canSee) > 0 {
		// Map the photo authors and return it.
		userMap, err := models.MapUsers(currentUser, userIDs)
		if err != nil {
			log.Error("AttachMarkdownGalleryLinksFooter: mapping photo owners: %s", err)
		}

		return canSee, userMap, nil
	}

	return nil, nil, nil
}
