package admin

import (
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// ChangeLog controller (/admin/changelog)
func ChangeLog() http.HandlerFunc {
	tmpl := templates.Must("admin/change_log.html")

	// Whitelist for ordering options.
	var sortWhitelist = []string{
		"created_at desc",
		"created_at asc",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Query parameters.
		var (
			tableName   = r.FormValue("table_name")
			tableID     uint64
			aboutUserID uint64
			aboutUser   = r.FormValue("about_user_id")
			adminUserID uint64
			adminUser   = r.FormValue("admin_user_id")
			event       = r.FormValue("event")
			sort        = r.FormValue("sort")
			searchQuery = r.FormValue("search")
			search      = models.ParseSearchString(searchQuery)
			sortOK      bool
		)

		// Sort options.
		for _, v := range sortWhitelist {
			if sort == v {
				sortOK = true
				break
			}
		}
		if !sortOK {
			sort = "created_at desc"
		}

		if i, err := strconv.Atoi(r.FormValue("table_id")); err == nil {
			tableID = uint64(i)
		}

		// User IDs can be string values to look up by username or email address.
		if aboutUser != "" {
			if i, err := strconv.Atoi(aboutUser); err == nil {
				aboutUserID = uint64(i)
			} else {
				if user, err := models.FindUsername(aboutUser); err == nil {
					aboutUserID = user.ID
				} else {
					session.FlashError(w, r, "Couldn't find About User ID: %s", err)
				}
			}
		}

		if adminUser != "" {
			if i, err := strconv.Atoi(adminUser); err == nil {
				adminUserID = uint64(i)
			} else {
				if user, err := models.FindUsername(adminUser); err == nil {
					adminUserID = user.ID
				} else {
					session.FlashError(w, r, "Couldn't find Admin User ID: %s", err)
				}
			}
		}

		pager := &models.Pagination{
			PerPage: config.PageSizeChangeLog,
			Sort:    sort,
		}
		pager.ParsePage(r)

		cl, err := models.PaginateChangeLog(tableName, tableID, aboutUserID, adminUserID, event, search, pager)
		if err != nil {
			session.FlashError(w, r, "Error paginating the change log: %s", err)
		}

		// Map the various user IDs.
		var (
			userIDs = []uint64{}
		)
		for _, row := range cl {
			if row.AboutUserID > 0 {
				userIDs = append(userIDs, row.AboutUserID)
			}
			if row.AdminUserID > 0 {
				userIDs = append(userIDs, row.AdminUserID)
			}
		}
		userMap, err := models.MapUsers(nil, userIDs)
		if err != nil {
			session.FlashError(w, r, "Error mapping user IDs: %s", err)
		}

		var vars = map[string]interface{}{
			"ChangeLog":  cl,
			"TableNames": models.ChangeLogTables(),
			"EventTypes": models.ChangeLogEventTypes,
			"Pager":      pager,
			"UserMap":    userMap,

			// Filters
			"TableName":   tableName,
			"TableID":     tableID,
			"AboutUserID": aboutUser,
			"AdminUserID": adminUser,
			"Event":       event,
			"SearchQuery": searchQuery,
			"Sort":        sort,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
