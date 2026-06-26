package account

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/controller/chat"
	"github.com/cuvou/gosocial/pkg/geoip"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/spam"
	"github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/utility"
	"github.com/cuvou/gosocial/pkg/worker"
)

// Search controller.
func Search() http.HandlerFunc {
	tmpl := templates.Must("account/search.html")

	// Whitelist for ordering options.
	var sortWhitelist = []string{
		"last_login_at desc",
		"created_at desc",
		"username",
		"username desc",
		"lower(name)",
		"lower(name) desc",
		"distance",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Search filters.
		var (
			isCertified     = r.FormValue("certified")
			username        = strings.TrimPrefix(strings.TrimSpace(r.FormValue("name")), "mailto:")
			searchTerm      = r.FormValue("search") // profile text search
			citySearch      = r.FormValue("wcs")
			gender          = r.FormValue("gender")
			orientation     = r.FormValue("orientation")
			maritalStatus   = r.FormValue("marital_status")
			hereFor         = r.FormValue("here_for")
			spokenLanguage  = r.FormValue("language")
			lastOnline, _   = strconv.Atoi(r.FormValue("last_online"))
			friendSearch    = r.FormValue("friends") == "true"
			likedSearch     = r.FormValue("liked") == "true"
			onChatSearch    = r.FormValue("on_chat") == "true"
			followerSearch  = r.FormValue("followers") == "true"
			followingSearch = r.FormValue("following") == "true"
			maxDistance     = r.FormValue("distance")
			sort            = utility.StringIn(r.FormValue("sort"), sortWhitelist, sortWhitelist[0])
		)

		ageMin, err1 := strconv.Atoi(r.FormValue("age_min"))
		ageMax, err2 := strconv.Atoi(r.FormValue("age_max"))
		if ageMin > ageMax && err1 == nil && err2 == nil {
			ageMin, ageMax = ageMax, ageMin
		}

		if lastOnline < 0 || lastOnline > 24*30 {
			lastOnline = 0
		}

		rawSearch := models.ParseSearchString(searchTerm)
		search, restricted := spam.RestrictSearchTerms(rawSearch)

		// Parse maxDistance into a float.
		distance, _ := strconv.ParseFloat(maxDistance, 64)

		// Get current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user!")
			templates.Redirect(w, "/")
			return
		}

		// Report when search terms are restricted.
		if restricted != nil {
			// Admin users: allow the search anyway.
			if currentUser.HasAdminScope(config.ScopeUserInsight) {
				search = rawSearch
			} else {
				fb := &models.Feedback{
					Intent:    "report",
					Subject:   "Search Keyword Blacklist",
					UserID:    currentUser.ID,
					TableName: "users",
					TableID:   currentUser.ID,
					Message: fmt.Sprintf(
						"A user has run a search on the Member Directory using search terms which are prohibited.\n\n"+
							"Their search query was: %s",
						searchTerm,
					),
				}

				// Save the feedback.
				if err := models.CreateFeedback(fb); err != nil {
					log.Error("Couldn't save feedback from user updating their DOB: %s", err)
				}
			}
		}

		// Geolocation/Who's Nearby: if the current user uses GeoIP, update
		// their coordinates now.
		var myLocation = &models.UserLocation{}
		if !session.Impersonated(r) {
			myLocation, err = models.RefreshGeoIP(currentUser.ID, r)
			if err != nil {
				log.Error("RefreshGeoIP: %s", err)
			}
		} else {
			myLocation = models.GetUserLocation(currentUser.ID)
		}

		// Are they doing a Location search (from world city typeahead)?
		var city *models.WorldCities
		if citySearch != "" {
			sort = "distance"

			// Require the current user to have THEIR location set, for fairness.
			if myLocation.Source == models.LocationSourceNone {
				session.FlashError(w, r, "You must set your own location before you can search for others by their location.")
			} else {
				// Look up the coordinates of their search.
				city, err = models.FindWorldCity(citySearch)
				if err != nil {
					session.FlashError(w, r, "Location search: no match was found for '%s', please use one of the exact search results from the type-ahead on the Location field.", citySearch)
					citySearch = "" // null out their search
				}
			}
		}

		// Real name for certified_at
		if sort == "certified_at desc" {
			sort = "certification_photos.updated_at desc"
		}

		// Default
		if isCertified == "" {
			isCertified = "true"
		}

		// Are we filtering for "On Chat?"
		var inUsername = []string{}
		if onChatSearch {
			stats := chat.FilteredChatStatistics(currentUser)
			inUsername = stats.Usernames
			if len(inUsername) == 0 {
				session.FlashError(w, r, "Notice: you wanted to filter by people currently on the chat room, but nobody is on chat at this time.")
				inUsername = []string{"@"}
			}
		}

		// Log the search terms for analytics.
		if searchTerm != "" {
			message := "Searched the member directory by keyword: " + searchTerm
			if restricted != nil {
				message += " (which was restricted)"
			}
			models.LogEvent(currentUser, nil, models.ChangeLogAnalytics, "users.search", 0, message)
		}

		pager := &models.Pagination{
			PerPage: config.PageSizeMemberSearch,
			Sort:    sort,
		}
		pager.ParsePage(r)

		users, err := models.SearchUsers(currentUser, &models.UserSearch{
			Username:       username,
			InUsername:     inUsername,
			Gender:         gender,
			Orientation:    orientation,
			MaritalStatus:  maritalStatus,
			HereFor:        hereFor,
			SpokenLanguage: spokenLanguage,
			ProfileText:    search,
			NearCity:       city,
			LastOnline:     lastOnline,
			MaxDistance:    distance,
			IsBanned:       isCertified == "banned",
			IsDisabled:     isCertified == "disabled",
			IsAdmin:        isCertified == "admin",
			IsAllUsers:     isCertified == "all",
			Friends:        friendSearch,
			Liked:          likedSearch,
			Followers:      followerSearch,
			Following:      followingSearch,
			AgeMin:         ageMin,
			AgeMax:         ageMax,
		}, pager)
		if err != nil {
			session.FlashError(w, r, "An error has occurred: %s.", err)
		}

		// Preload all of their profile fields, so their hometown/pronouns/etc. are available.
		if err := models.PreloadUserProfileFields(users); err != nil {
			log.Error("Preloading %d users' profile fields: %s", len(users), err)
		}

		// Who's Nearby feature, get some data.
		insights, _ := geoip.GetRequestInsights(r)

		// Collect usernames to map to chat online status.
		var usernames = []string{}
		var userIDs = []uint64{}
		for _, user := range users {
			usernames = append(usernames, user.Username)
			userIDs = append(userIDs, user.ID)
		}

		// User IDs of these I have "Liked"
		likedIDs, err := models.LikedIDs(currentUser, "users", userIDs)
		if err != nil {
			log.Error("LikedIDs: %s", err)
		}

		var vars = map[string]interface{}{
			"Users": users,
			"Pager": pager,

			"Enum":        config.ProfileEnums,
			"HereForEnum": config.HereFor,

			// Search filter values.
			"Certified":       isCertified,
			"Gender":          gender,
			"Orientation":     orientation,
			"MaritalStatus":   maritalStatus,
			"HereFor":         hereFor,
			"SpokenLanguage":  spokenLanguage,
			"EmailOrUsername": username,
			"Search":          searchTerm,
			"City":            citySearch,
			"AgeMin":          ageMin,
			"AgeMax":          ageMax,
			"LastOnline":      lastOnline,
			"FriendSearch":    friendSearch,
			"LikedSearch":     likedSearch,
			"FollowerSearch":  followerSearch,
			"FollowingSearch": followingSearch,
			"OnChatSearch":    onChatSearch,
			"FilterDistance":  maxDistance,
			"Sort":            sort,

			// Restricted Search errors.
			"RestrictedSearchError": restricted,

			// Photo counts mapped to users
			"PhotoCountMap": models.MapPhotoCounts(users),

			// Map friendships and likes to these users.
			"FriendMap": models.MapFriends(currentUser, users),
			"LikedMap":  models.MapLikes(currentUser, "users", likedIDs),
			"FollowMap": models.MapFollows(currentUser, userIDs),

			// Users on the chat room map.
			"UserOnChatMap": worker.GetChatStatistics().MapUsersOnline(usernames),

			// Current user's location setting.
			"MyLocation":       myLocation,
			"GeoIPInsights":    insights,
			"DistanceMap":      models.MapDistances(currentUser, city, users),
			"DistanceLimiters": config.DistanceLimiters,
		}

		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
