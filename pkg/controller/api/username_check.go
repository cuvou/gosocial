package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cuvou/gosocial/pkg/models"
)

// UsernameCheck API.
func UsernameCheck() http.HandlerFunc {
	// Request JSON schema.
	type Request struct {
		Username string `json:"username"`
	}

	// Response JSON schema.
	type Response struct {
		OK    bool   `json:"OK"`
		Error string `json:"error,omitempty"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			SendJSON(w, http.StatusNotAcceptable, Response{
				Error: "POST method only",
			})
			return
		}

		// Parse request payload.
		var req Request
		if err := ParseJSON(r, &req); err != nil {
			SendJSON(w, http.StatusBadRequest, Response{
				Error: fmt.Sprintf("Error with request payload: %s", err),
			})
			return
		}

		// Username to test.
		var username = strings.TrimSpace(strings.ToLower(req.Username))
		if err := models.IsValidUsername(username); err != nil {
			SendJSON(w, http.StatusOK, Response{
				Error: err.Error(),
			})
			return
		}

		// Send success response.
		SendJSON(w, http.StatusOK, Response{
			OK: true,
		})
	})
}
