package admin

import (
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/geoip"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// IP Address Insights.
func IPInsights() http.HandlerFunc {
	tmpl := templates.Must("admin/ip_insights.html")

	var sortWhitelist = []string{
		"updated_at desc",
		"created_at desc",
		"number_visits desc",

		"last_login_at desc",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var (
			intUserID, _ = strconv.Atoi(r.FormValue("user_id"))
			userID       = uint64(intUserID)
			sort         = r.FormValue("sort")
			sortOK       bool
			addr         = r.FormValue("ip")
		)

		for _, s := range sortWhitelist {
			if sort == s {
				sortOK = true
				break
			}
		}
		if !sortOK {
			sort = sortWhitelist[0]
		}

		// Searching by user ID?
		var user *models.User
		if userID > 0 {
			if u, err := models.GetUser(userID); err != nil {
				session.FlashError(w, r, "Couldn't find user ID %d: %s", userID, err)
				templates.Redirect(w, "/admin")
				return
			} else {
				user = u
			}
		}

		// Pagers.
		var pager = &models.Pagination{
			PerPage: config.PageSizeAdminIPInsights,
			Sort:    sort,
		}
		pager.ParsePage(r)

		// Second pager (dashboard).
		var pager2 = &models.Pagination{
			PerPage: config.PageSizeAdminIPInsights,
			Sort:    sort,
		}
		pager2.ParsePage(r)

		// Paginating the user's IP addresses?
		var userIPs []*models.IPAddress
		if user != nil {
			if ips, err := models.PaginateIPsByUser(user, pager); err != nil {
				session.FlashError(w, r, "Error paginating user IP addresses: %s", err)
			} else {
				userIPs = ips
			}
		}

		// Paginating users by IP address?
		var users []*models.User
		if addr != "" {
			if u, err := models.PaginateUsersByIP(addr, pager); err != nil {
				session.FlashError(w, r, "Error paginating users by IP address: %s", err)
			} else {
				users = u
			}
		}

		// Dashboard view (not inspecting a user or IP)
		var (
			bannedIPs []*models.IPAddress
			sharedIPs []*models.SharedIPInsights
		)
		if user == nil && addr == "" {
			if ips, err := models.PaginateSharedIPs(pager); err != nil {
				session.FlashError(w, r, "Error paginating shared IPs: %s", err)
			} else {
				sharedIPs = ips
			}

			if ips, err := models.PaginateBannedIPs(pager2); err != nil {
				session.FlashError(w, r, "Error paginating banned IPs: %s", err)
			} else {
				bannedIPs = ips
			}
		}

		// Map GeoIP insights.
		var ipAddresses []string
		for _, ip := range userIPs {
			ipAddresses = append(ipAddresses, ip.IPAddress)
		}
		for _, ip := range bannedIPs {
			ipAddresses = append(ipAddresses, ip.IPAddress)
		}
		for _, ip := range sharedIPs {
			ipAddresses = append(ipAddresses, ip.IPAddress)
		}
		insightsMap := geoip.MapInsights(ipAddresses)
		bannedMap := models.MapBannedIPs(ipAddresses)

		vars := map[string]any{
			// Filter params: user or IP?
			"User":    user,
			"Address": addr,

			// Paginated view for filtered (user IPs, or users per IP)
			"IPs":   userIPs,
			"Users": users,

			// Map GeoIP and ban status insights.
			"InsightsMap": insightsMap,
			"BannedMap":   bannedMap,

			// Dashboard paginations.
			"BannedIPs": bannedIPs,
			"SharedIPs": sharedIPs,

			"Pager":  pager,
			"Pager2": pager2,
			"Sort":   sort,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
