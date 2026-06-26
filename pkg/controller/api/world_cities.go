package api

import (
	"fmt"
	"net/http"

	"github.com/cuvou/gosocial/pkg/models"
)

// WorldCities API searches the location database for a world city location.
func WorldCities() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var query = r.FormValue("query")
		if query == "" {
			SendRawJSON(w, http.StatusOK, []*models.WorldCities{})
			return
		}

		result, err := models.SearchWorldCities(query)
		if err != nil {
			SendRawJSON(w, http.StatusInternalServerError, []*models.WorldCities{{
				ID:        1,
				Canonical: err.Error(),
			}})
			return
		}

		SendRawJSON(w, http.StatusOK, result)
	})
}

// WorldCitiesPretty API returns the pretty country flag/location name for a geo coordinate.
func WorldCitiesPretty() http.HandlerFunc {
	type Request struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	}
	type Response struct {
		OK     bool   `json:"ok"`
		Result string `json:"result,omitempty"`
		Error  string `json:"error,omitempty"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Parse request payload.
		var req Request
		if err := ParseJSON(r, &req); err != nil {
			SendJSON(w, http.StatusBadRequest, Response{
				Error: fmt.Sprintf("Error with request payload: %s", err),
			})
			return
		}

		result, err := models.NearestWorldCity(req.Latitude, req.Longitude)
		if err != nil {
			SendJSON(w, http.StatusBadRequest, Response{
				Error: err.Error(),
			})
			return
		}

		emoji, err := result.PrettyEmojiString()
		if err != nil {
			SendJSON(w, http.StatusBadRequest, Response{
				Error: err.Error(),
			})
			return
		}

		SendJSON(w, http.StatusOK, Response{
			OK:     true,
			Result: emoji,
		})
	})
}
