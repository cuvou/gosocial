package templates

import (
	"net/http"
	"net/url"
	"time"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/worker"
)

// MergeVars mixes in globally available template variables. The http.Request is optional.
func MergeVars(r *http.Request, m map[string]interface{}) {
	m["Title"] = config.Title
	m["WebsiteURL"] = config.WebsiteURL
	m["BuildHash"] = config.RuntimeBuild
	m["BuildDate"] = config.RuntimeBuildDate
	m["Subtitle"] = config.Subtitle
	m["YYYY"] = time.Now().Year()
	m["WebsiteTheme"] = ""
	m["StaticPhotoBaseURL"] = config.PhotoWebPath

	// Integrations
	m["TurnstileCAPTCHA"] = config.Current.Turnstile

	// Feature flags
	m["FeatureUserForumsEnabled"] = config.UserForumsEnabled

	// Global upload settings.
	m["MaxBodyMegaBytes"] = config.MaxBodyMegaBytes
	m["MaxBodyBytes"] = config.MaxBodyBytes

	// Global maintenance mode settings.
	m["SiteMaintenanceMode"] = config.Current.Maintenance
	m["SiteEmergencyMode"] = config.Current.EmergencyKillSwitch

	if r == nil {
		return
	}

	m["Request"] = r
	m["RequestFullURIEscaped"] = url.QueryEscape(r.URL.String())
}

// MergeUserVars mixes in global template variables: LoggedIn and CurrentUser. The http.Request is optional.
func MergeUserVars(r *http.Request, m map[string]interface{}) {
	// Defaults
	m["LoggedIn"] = false
	m["CurrentUser"] = nil
	m["SessionImpersonated"] = false

	// User notification counts for nav bar.
	m["NavUnreadMessages"] = 0      // New messages
	m["NavFriendRequests"] = 0      // Friend requests
	m["NavUnreadFootprints"] = 0    // unread footprints
	m["NavUnreadNotifications"] = 0 // general notifications
	m["NavTotalNotifications"] = 0  // Total of above
	m["NavChatStatistics"] = worker.GetChatStatistics()

	// Admin notification counts for nav bar.
	m["NavCertificationPhotos"] = 0    // Cert. photos needing approval
	m["NavContentApprovals"] = 0       // Content approvals pending
	m["NavAdminFeedback"] = 0          // Unacknowledged feedback
	m["NavAppealedExplicitPhotos"] = 0 // Appealed explicit photos
	m["NavAdminNotifications"] = 0     // Total of above

	// Mobile bottom nav Overflow menu notification counter.
	m["NavMobileOverflowNotifications"] = 0

	if r == nil {
		return
	}

	m["SessionImpersonated"] = session.Impersonated(r)

	if user, err := session.CurrentUser(r); err == nil {
		m["LoggedIn"] = true
		m["CurrentUser"] = user

		// User website preferences
		m["WebsiteTheme"] = user.GetProfileField("website-theme")

		// Get user recent notifications.
		/*notifPager := &models.Pagination{
			Page:    1,
			PerPage: 10,
		}
		if notifs, err := models.PaginateNotifications(user, notifPager); err == nil {
			m["Notifications"] = notifs
		}*/

		// Collect notification counts.
		var (
			// For users
			countMessages      int64
			countFriendReqs    int64
			countNotifications int64

			// For admins
			countFeedback int64
		)

		// Get unread message count.
		if count, err := models.CountUnreadMessages(user); err == nil {
			m["NavUnreadMessages"] = count
			countMessages = count
		} else {
			log.Error("MergeUserVars: couldn't CountUnreadMessages for %d: %s", user.ID, err)
		}

		// Get friend request count.
		if count, err := models.CountFriendRequests(user.ID); err == nil {
			m["NavFriendRequests"] = count
			countFriendReqs = count
		} else {
			log.Error("MergeUserVars: couldn't CountFriendRequests for %d: %s", user.ID, err)
		}

		// Count other notifications.
		if count, err := models.CountUnreadNotifications(user); err == nil {
			m["NavUnreadNotifications"] = count
			countNotifications = count
		} else {
			log.Error("MergeUserVars: couldn't CountFriendRequests for %d: %s", user.ID, err)
		}

		// Are we admin? Add notification counts if the current admin can respond to them.
		if user.IsAdmin {

			// Admin feedback available?
			if user.HasAdminScope(config.ScopeFeedbackAndReports) {
				countFeedback = models.CountUnreadFeedback()
			}

			m["NavAdminFeedback"] = countFeedback

			// Total notification count for admin actions.
			m["NavAdminNotifications"] = countFeedback
		}

		// Total count for user notifications.
		m["NavTotalNotifications"] = countMessages + countFriendReqs + countNotifications + countFeedback
		m["NavMobileOverflowNotifications"] = countFriendReqs + countFeedback
	}
}
