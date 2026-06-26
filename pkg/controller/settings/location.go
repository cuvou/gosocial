package settings

import (
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/geoip"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Location settings (/settings/location).
func Location() http.HandlerFunc {
	tmpl := templates.Must("settings/location.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Load the current user in case of updates.
		user, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get CurrentUser: %s", err)
			templates.Redirect(w, r.URL.Path)
			return
		}

		// Are we POSTing?
		if r.Method == http.MethodPost {
			var (
				source = r.PostFormValue("source")
				latStr = r.PostFormValue("latitude")
				lonStr = r.PostFormValue("longitude")
			)

			// Get and update the user's location.
			location := models.GetUserLocation(user.ID)
			location.Source = source

			if lat, err := strconv.ParseFloat(latStr, 64); err == nil {
				location.Latitude = lat
			} else {
				location.Latitude = 0
			}

			if lon, err := strconv.ParseFloat(lonStr, 64); err == nil {
				location.Longitude = lon
			} else {
				location.Longitude = 0
			}

			// Save it.
			if err := location.Save(); err != nil {
				session.FlashError(w, r, "Couldn't save your location preference: %s", err)
			} else {
				session.Flash(w, r, "Location settings updated!")
			}

			templates.Redirect(w, r.URL.Path)
			return
		}

		// For the Location tab: get GeoIP insights.
		insights, err := geoip.GetRequestInsights(r)
		if err != nil {
			log.Error("GetRequestInsights: %s", err)
		}

		vars := map[string]interface{}{
			"GeoIPInsights": insights,
			"UserLocation":  models.GetUserLocation(user.ID),
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
