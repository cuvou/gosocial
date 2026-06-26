package templates

import (
	"fmt"
	"net/http"
)

// Redirect sends an HTTP header to the browser.
func Redirect(w http.ResponseWriter, url string) {
	w.Header().Set("Location", url)
	w.WriteHeader(http.StatusFound)
}

/*
RedirectRoute redirects an old URL route to a newer version.

This was added for the Go 1.22 path parameter update to the standard lib
router. Before this update, routes with path parameters were handled by
regexp parsing inside the controller functions and I didn't want to overload
too many endpoints sharing a common prefix but with 1.22 path parameters
this is easier to do.

Examples:

* /u/{username}/friends instead of /friends/u/{username}
* /u/{username}/notes instead of /notes/u/{username}
*/
func RedirectRoute(path string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var a = r.PathValue("s")
		if a != "" {
			Redirect(w, fmt.Sprintf(path, a))
			return
		}
		Redirect(w, path)
	}
}
