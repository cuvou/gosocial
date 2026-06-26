package api

import (
	"net/http"
	"strings"

	"github.com/tkuchiki/go-timezone"
)

// TimeZones API searches the tzinfo database for an IANA time zone code.
func TimeZones() http.HandlerFunc {
	type Result struct {
		ShortCode string `json:"shortCode"`
		Value     string `json:"value"`
	}

	tzinfo := timezone.New()

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var query = strings.ToLower(r.FormValue("query"))
		if query == "" {
			SendRawJSON(w, http.StatusOK, []Result{})
			return
		}

		// Replace spaces with underscores (e.g. "Los Angeles" => "America/Los_Angeles")
		query = strings.ReplaceAll(query, " ", "_")

		// Search the time zones.
		var (
			result   = []Result{}
			distinct = map[string]struct{}{}
		)
		tz := tzinfo.Timezones()
		for shortCode, names := range tz {
			for _, iana := range names {
				if _, ok := distinct[iana]; ok {
					continue
				}
				distinct[iana] = struct{}{}

				if strings.Contains(strings.ToLower(iana), query) {
					result = append(result, Result{
						ShortCode: shortCode,
						Value:     iana,
					})
				}
			}

			if len(result) > 10 {
				break
			}
		}

		SendRawJSON(w, http.StatusOK, result)
	})
}
