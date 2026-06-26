package api

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
)

// ReadNotification API to mark a notif ID as "read."
func ReadNotification() http.HandlerFunc {
	// Request JSON schema.
	type Request struct {
		ID string `json:"id"`
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

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			SendJSON(w, http.StatusBadRequest, Response{
				Error: "Couldn't get current user!",
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

		// Parse IDs to integers.
		var IDs []uint64
		for _, strID := range strings.Split(req.ID, ",") {
			if a, err := strconv.Atoi(strID); err == nil {
				IDs = append(IDs, uint64(a))
			}
		}

		if len(IDs) == 0 {
			SendJSON(w, http.StatusInternalServerError, Response{
				Error: "No IDs given",
			})
			return
		}

		// Mark them read.
		if err := models.MarkSpecificNotificationsRead(currentUser, IDs); err != nil {
			SendJSON(w, http.StatusInternalServerError, Response{
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

// ClearNotification API to delete a single notification for the user.
func ClearNotification() http.HandlerFunc {
	// Request JSON schema.
	type Request struct {
		ID string `json:"id"`
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

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			SendJSON(w, http.StatusBadRequest, Response{
				Error: "Couldn't get current user!",
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

		// Parse IDs to integers.
		var IDs []uint64
		for _, strID := range strings.Split(req.ID, ",") {
			if a, err := strconv.Atoi(strID); err == nil {
				IDs = append(IDs, uint64(a))
			}
		}

		if len(IDs) == 0 {
			SendJSON(w, http.StatusInternalServerError, Response{
				Error: "No IDs given",
			})
			return
		}

		// Clear these notifications.
		if err := models.ClearSpecificNotifications(currentUser, IDs); err != nil {
			SendJSON(w, http.StatusInternalServerError, Response{
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
