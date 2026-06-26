package comment

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// GoToTaggedUser finds the correct link to view content you were tagged in.
func GoToTaggedUser() http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Query params.
		var (
			tableName  = r.FormValue("table")
			tableID, _ = strconv.ParseUint(r.FormValue("id"), 10, 64)
		)

		switch tableName {
		case "photos":
			templates.Redirect(w, fmt.Sprintf("/photo/view?id=%d", tableID))
			return
		}

		session.FlashError(w, r, "Couldn't find the right page to link you to. This is likely to be a bug, please let an admin know.")
		templates.Redirect(w, "/me")
	})
}
