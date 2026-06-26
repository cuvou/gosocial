package comment

import (
	"fmt"
	"net/http"

	"github.com/cuvou/gosocial/pkg/templates"
)

// GoToPhoto redirects to the photo view permalink.
//
// This is mainly useful to link to a photo without using the standard URL, so that it doesn't
// get embedded and attached to a post.
func GoToPhoto() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var idStr = r.FormValue("id")
		templates.Redirect(w, fmt.Sprintf("/photo/view?id=%s", idStr))
	})
}
