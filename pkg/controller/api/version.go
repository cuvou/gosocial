package api

import (
	"encoding/json"
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/utility"
)

// Version details of the running app.
func Version() http.HandlerFunc {
	// Response JSON schema.
	type Response struct {
		Version   string `json:"version"`
		Build     string `json:"build"`
		BuildDate string `json:"buildDate"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, err := json.Marshal(Response{
			Version:   config.RuntimeVersion,
			Build:     config.RuntimeBuild,
			BuildDate: config.RuntimeBuildDate,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(buf)
	})
}

// TestPanic triggers a panic to test recovery procedures.
func TestPanic() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("Testing panic recovery!")
	})
}

// Debug details of the running app.
func Debug() http.HandlerFunc {
	// Response JSON schema.
	type ResponseRequest struct {
		RemoteAddr      string
		XForwardedFor   string
		XRealIP         string
		UserAgent       string
		ParsedIPAddress string
	}
	type Response struct {
		Request ResponseRequest
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		buf, err := json.Marshal(Response{
			Request: ResponseRequest{
				RemoteAddr:      r.RemoteAddr,
				XForwardedFor:   r.Header.Get("X-Forwarded-For"),
				XRealIP:         r.Header.Get("X-Real-IP"),
				UserAgent:       r.UserAgent(),
				ParsedIPAddress: utility.IPAddress(r),
			},
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.Write(buf)
	})
}
