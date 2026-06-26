package api

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/markdown"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
)

// SearchUsernames drives auto-complete type-ahead search for usernames.
func SearchUsernames() http.HandlerFunc {
	type Result struct {
		Label string `json:"label"`
		Value string `json:"value"`
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			query    = strings.ToLower(strings.TrimSpace(r.FormValue("query")))
			multiple = r.FormValue("multiple") == "true"
		)
		if query == "" {
			SendRawJSON(w, http.StatusOK, []Result{})
			return
		}

		// If doing a multiple-input search, only consider the last value.
		if multiple {
			parts := strings.Split(query, ",")
			query = strings.TrimSpace(parts[len(parts)-1])
		}

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			SendRawJSON(w, http.StatusOK, []Result{
				{
					Value: fmt.Sprintf("Couldn't get current user: %s", err),
				},
			})
			return
		}

		// Search the member directory.
		users, err := models.SearchUsers(currentUser, &models.UserSearch{
			Username: query,
		}, &models.Pagination{
			PerPage: config.PageSizeUsernameTypeAhead,
			Sort:    "username asc",
		})

		var res = []Result{}
		for _, user := range users {

			// Format their item (with display label if set).
			label := fmt.Sprintf("@%s", user.Username)
			if user.Name != nil && len(*user.Name) > 0 {
				label = fmt.Sprintf(
					`%s <small class="is-size-7">(@%s)</small>`,
					markdown.StripHTML(*user.Name),
					user.Username,
				)
			}
			res = append(res, Result{
				Label: label,
				Value: user.Username,
			})
		}

		SendRawJSON(w, http.StatusOK, res)
	})
}
