package api

import (
	"fmt"
	"net/http"

	"github.com/cuvou/gosocial/pkg/markdown"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// MarkdownPreview API reflects Markdown rendered text.
func MarkdownPreview() http.HandlerFunc {
	// Request JSON schema.
	type Request struct {
		Text         string `json:"text"`
		GalleryLinks string `json:"galleryLinks"`
	}

	// Response JSON schema.
	type Response struct {
		OK    bool   `json:"OK"`
		Error string `json:"error,omitempty"`
		HTML  string `json:"html"`
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

		// Validation.
		if req.Text == "" {
			req.Text = `<small><em>No text was written to be previewed!</em></small>`
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			SendJSON(w, http.StatusBadRequest, Response{
				Error: "Error: Not logged in.",
			})
			return
		}

		var rendered string

		// Supporting gallery links?
		switch req.GalleryLinks {
		case "inline":
			rendered = string(templates.ToMarkdownWithGalleryLinksInline(r, currentUser, req.Text))
		case "footer":
			rendered = markdown.Render(req.Text)
			footer := templates.AttachMarkdownGalleryLinksFooter(r, currentUser, rendered)
			rendered += string(footer)
		default:
			rendered = markdown.Render(req.Text)
		}

		// Send success response.
		SendJSON(w, http.StatusOK, Response{
			OK:   true,
			HTML: rendered,
		})
	})
}
