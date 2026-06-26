package htmx

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cuvou/gosocial/pkg/middleware"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// LoadGalleryEmbeds loads photos attached to forum comments and messages.
func LoadGalleryEmbeds() http.HandlerFunc {
	tmpl := templates.MustLoadCustom("partials/htmx/gallery_embeds.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			w.Write([]byte("You must be logged in to view this page."))
			return
		}

		// Is the site under a Maintenance Mode restriction?
		if middleware.MaintenanceMode(currentUser, w, r) {
			return
		}

		// Parse the photo IDs from the query string.
		var (
			strIDs   = strings.Split(r.FormValue("ids"), ",")
			photoIDs = []uint64{}
		)
		for _, strID := range strIDs {
			if i, err := strconv.Atoi(strID); err == nil {
				photoIDs = append(photoIDs, uint64(i))
			}
		}

		if len(photoIDs) == 0 {
			w.Write([]byte("No photos asked for."))
			return
		}

		if photos, userMap, err := templates.GetFilteredEmbeddedPhotos(currentUser, photoIDs); err == nil {
			var (
				buf  = bytes.NewBuffer([]byte{})
				vars = map[string]interface{}{
					"Photos":  photos,
					"UserMap": userMap,
				}
			)
			if err := tmpl.Execute(buf, r, vars); err != nil {
				fmt.Fprintf(w, "[template error: %s]", err)
				return
			}

			w.Write(buf.Bytes())
			return
		}
	})
}
