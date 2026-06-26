package config

import "regexp"

// Various hard-coded enums such as choice of gender, sexuality, relationship status etc.
var (
	MaritalStatus = []string{
		"Single",
		"Married",
		"In a relationship",
		"It's complicated",
		"Divorced",
		"Widowed",
		"Widower",
	}

	RelationshipType = []string{
		"Monogamous",
		"Open",
	}

	Gender = []string{
		"Man",
		"Woman",
		"Non-binary",
		"Trans",
		"Trans (FTM)",
		"Trans (MTF)",
		"Other",
	}

	// Subset of Trans genders, to aid the member directory search: if they search for Trans
	// also include all subcategories.
	TransGender = []string{
		"Trans",
		"Trans (FTM)",
		"Trans (MTF)",
	}

	Orientation = []string{
		"Straight",
		"Gay",
		"Bisexual",
		"Bicurious",
		"Pansexual",
		"Asexual",
	}

	HereFor = []OptGroup{
		{
			Header: "Meeting Others",
			Options: []Option{
				{Value: "Dating"},
				{Value: "Relationship"},
				{Value: "Platonic friends"},
				{Value: "Travel companions"},
				{Value: "Networking"},
				{Value: "Casual acquaintances"},
			},
		},
		{
			Header: "Site Features",
			Options: []Option{
				{Value: "Chat room"},
				{Value: "Forums"},
				{Value: "Galleries"},
			},
		},
		{
			Header: "Recreational Activities",
			Options: []Option{
				{Value: "Couch surfing"},
				{Value: "Bicycling"},
				{Value: "Hiking"},
				{Value: "Backpacking"},
				{Value: "Swimming"},
				{Value: "Camping"},
				{Value: "Sunbathing"},
				{Value: "Pickleball"},
				{Value: "Gaming (video games)"},
				{Value: "Ping pong"},
				{Value: "Pot lucks"},
				{Value: "Snow skiing"},
				{Value: "Snow shoeing"},
			},
		},
	}

	SpokenLanguages = []string{
		"English",
		"Arabic",
		"Bulgarian",
		"Catalan",
		"Chinese",
		"Croatian",
		"Czech",
		"Danish",
		"Dutch",
		"Esperanto",
		"Estonian",
		"Finnish",
		"French",
		"German",
		"Greek",
		"Hebrew",
		"Hungarian",
		"Indonesian",
		"Irish",
		"Italian",
		"Japanese",
		"Korean",
		"Latvian",
		"Lithuanian",
		"Norwegian",
		"Persian",
		"Polish",
		"Portuguese",
		"Romanian",
		"Russian",
		"Serbian",
		"Sign language",
		"Slovak",
		"Slovenian",
		"Spanish",
		"Swedish",
		"Thai",
		"Turkish",
		"Ukrainian",
		"Vietnamese",
		"Welsh",
	}

	// Enums all wrapped up for template use.
	ProfileEnums = map[string][]string{
		"MaritalStatus":    MaritalStatus,
		"RelationshipType": RelationshipType,
		"Gender":           Gender,
		"Orientation":      Orientation,
		"SpokenLanguages":  SpokenLanguages,
	}

	// Input field names for profile fields (freeform entry).
	ProfileFields = []string{
		"pronouns",
		"city",
		"job",
		"hide_age",
	}
	EnumProfileFields = map[string][]string{
		// Profile fields with limited enum options (validated at form submit time).
		"gender":            Gender,
		"orientation":       Orientation,
		"marital_status":    MaritalStatus,
		"relationship_type": RelationshipType,
	}
	EssayProfileFields = []string{
		"about_me",
		"interests",
		"music_movies",
	}

	// Site preference names (stored in ProfileField table).
	// NOTE: This enum is not used in code but is kept in sync to document the various profile
	// fields in use throughout the site.
	SitePreferenceFields = []string{
		"dm_privacy",
		"blur_explicit",
		"site_gallery_default",          // default view on site gallery (friends-only or all certified?)
		"notification_grouping_default", // group/ungroup their dashboard notifications (bool)
		"security_checkup_eligible",     // if true, show the security checkup page. Updates on RefreshLoginAt.
		"security_checkup_not_before",   // Security Checkup cooldown timers.
		"security_checkup_not_before_soft",
		"forum_newest_default", // default "Which forum?" selection.
		"forum_newest_photos",  // photo visibility status for Newest forum page.
	}

	// Privacy Setting enum acceptable values.
	PrivacySettingFirstMessages = []string{
		"", "friends", "nobody",
	}
	PrivacySettingPhotoComments = []string{
		"", "friends", "nobody",
	}
	PrivacySettingPrivatePhotos = []string{
		"", "friends", "messaged", "nobody",
	}
	PrivacySettingFriendsList = []string{
		"", "friends", "me",
	}
	PrivacySettingFollowMe = []string{
		"", "friends",
	}

	// Website theme color hue choices.
	WebsiteThemeHueChoices = []OptGroup{
		{
			Header: "Custom Themes",
			Options: []Option{
				{
					Label: "Bulma (no added color; classic gosocial theme)",
					Value: "none", // special value 'none' applies no extra CSS theme
				},
				{
					Label: "Default: blue & pink",
					Value: "blue-pink",
				},
			},
		},
		{
			Header: "Just a Splash of Color",
			Options: []Option{
				{
					Label: "Burnt red",
					Value: "red",
				},
				{
					Label: "Harvest orange",
					Value: "orange",
				},
				{
					Label: "Golden yellow",
					Value: "yellow",
				},
				{
					Label: "Leafy green",
					Value: "green",
				},
				{
					Label: "Cool blue",
					Value: "blue",
				},
				{
					Label: "Pretty in pink",
					Value: "pink",
				},
				{
					Label: "Royal purple",
					Value: "purple",
				},
				{
					Label: "Muted grey",
					Value: "grey",
				},
			},
		},
		{
			Header: "Dynamic",
			Options: []Option{
				{
					Label: "A different color every day",
					Value: "rotate-colors",
				},
			},
		},
	}
	WebsiteThemeRotateColors = []string{
		// The rotate-colors theme options.
		"blue-pink", "red", "orange", "yellow", "green", "blue", "pink", "purple", "grey",
	}

	// Choices for the Contact Us subject
	ContactUsChoices = []OptGroup{
		{
			Header: "Website Feedback",
			Options: []Option{
				{"feedback", "Website feedback"},
				{"feature", "Make a feature request"},
				{"bug", "Report a bug or broken feature"},
				{"2FA", "Problem logging into my account or with Two Factor Auth"},
				{"billing", "Billing, payments & the Paid Supporter Tier"},
				{"other", "General/miscellaneous/other"},
			},
		},
		{
			Header: "Report a Problem",
			Options: []Option{
				{"report.user", "Report a problematic user"},
				{"report.photo", "Report a problematic photo"},
				{"report.message", "Report a direct message conversation"},
				{"report.comment", "Report a forum post or comment"},
				{"report.forum", "Report a forum or community"},
			},
		},
	}

	// Default forum categories for forum landing page.
	ForumCategories = []string{
		"Rules and Announcements",
		"General",
		"Photo Boards",
		"Anything Goes",
	}

	// Blog categories.
	BlogCategories = []string{
		"Art and Photography",
		"Automotive",
		"Blogging",
		"Books and Stories",
		"Dreams and the Supernatural",
		"Fashion, Style, Shopping",
		"Food and Restaurants",
		"Friends",
		"Games",
		"Goals, Plans, Hopes",
		"Jobs, Work, Careers",
		"Life",
		"Movies, TV, Celebrities",
		"Music",
		"News and Politics",
		"gosocial",
		"Parties and Nightlife",
		"Pets and Animals",
		"Podcast",
		"Quiz/Survey",
		"Religion and Philosophy",
		"Romance and Relationships",
		"School, College, University",
		"Sports",
		"Travel and Places",
		"Web, HTML, Tech",
		"Writing and Poetry",
	}

	// Place categories and orientations.
	PlaceCategories = []string{
		"Bar/Club",
		"Beach",
		"Camp Ground",
		"Hot Springs",
		"Lake",
		"Offsite",
		"Organization",
		"Park",
		"Pool",
		"River",
		"Resort",
		"Spa",
		"Trail",
		"Other",
	}
	PlaceAmenities = []string{
		"Bar",
		"Basketball",
		"Billiards",
		"BYOB",
		"Clubhouse",
		"Fishing",
		"Game room",
		"Golf",
		"Gym",
		"Hiking",
		"Horseshoes",
		"Internet",
		"Kids Play Area",
		"Massage",
		"Nightclub",
		"Onsite store",
		"Pickelball",
		"Pool",
		"Restaurant",
		"Sauna",
		"Serves alcohol",
		"Shuffleboard",
		"Spa",
		"Swimming",
		"Tennis",
		"Volleyball",
		"WiFi",
		"Yoga",
	}
	PlaceOrientations = []string{
		"Family",
		"Couples",
		"Mixed",
		"Adults only",
		"Men only",
		"Women only",
		"Single men",
		"Single women",
		"Children",
		"LGBTQ+",
	}
	PlaceAccommodations = []string{
		"Cabin",
		"Condo rentals",
		"Premium room",
		"RV hookups",
		"Standard room",
		"Tent camping",
	}

	// Forum Poll expiration options.
	PollExpires = []Option{
		{
			Label: "Never",
			Value: "0",
		},
		{
			Label: "1 Day",
			Value: "1",
		},
		{
			Label: "2 Days",
			Value: "2",
		},
		{
			Label: "3 Days",
			Value: "3",
		},
		{
			Label: "4 Days",
			Value: "4",
		},
		{
			Label: "5 Days",
			Value: "5",
		},
		{
			Label: "6 Days",
			Value: "6",
		},
		{
			Label: "7 Days",
			Value: "7",
		},
		{
			Label: "2 Weeks",
			Value: "14",
		},
		{
			Label: "1 Month (30 days)",
			Value: "30",
		},
	}

	// Distance limiters.
	DistanceLimiters = []Option{
		{
			Label: "1.6km / 1mi",
			Value: "1.609344",
		},
		{
			Label: "8km / 5mi",
			Value: "8.04672",
		},
		{
			Label: "16km / 10mi",
			Value: "16.09344",
		},
		{
			Label: "40.2km / 25mi",
			Value: "40.2336",
		},
		{
			Label: "80.5km / 50mi",
			Value: "80.4672",
		},
		{
			Label: "120.7km / 75mi",
			Value: "120.7008",
		},
		{
			Label: "160.9km / 100mi",
			Value: "160.9344",
		},
		{
			Label: "241.4km / 150mi",
			Value: "241.4016",
		},
		{
			Label: "321.9km / 200mi",
			Value: "321.8688",
		},
		{
			Label: "482.8km / 300mi",
			Value: "482.8032",
		},
		{
			Label: "804.7km / 500mi",
			Value: "804.672",
		},
		{
			Label: "1609.3km / 1000mi",
			Value: "1609.344",
		},
	}

	// Keywords that appear in a DM that make it likely spam.
	DirectMessageSpamKeywords = []*regexp.Regexp{
		regexp.MustCompile(`\b(telegram|whats\s*app|signal|kik|session|google talk|google chat|gtalk|gchat)\b`),
		regexp.MustCompile(`https?://(t.me|join.skype.com|zoom.us|whereby.com|meet.jit.si|wa.me)`),
	}

	// Admin User Notes: to empower chat moderators, the Notes tab on user profile pages will show a
	// limited view of the Feedback & Reports about a user, as relevant to the chat room. This is the
	// whitelist of "Message" substrings to show on this page to low-privileged admins. Note: this whitelist
	// should be limited to the least sensitive reports.
	HighSensitivityAdminFeedbackSubjects = []string{
		"Explicit photo flag dispute",
		"Search Keyword Blacklist",
	}
	HighSensitivityAdminFeedbackSubstrings = []string{
		`%* User comment: This is an automated report via server side chat filters.%`,
	}

	// Static file extension suffixes: avoid users having a username end with these so we don't cause
	// any cache problems with Cloudflare thinking their profile URL is a static file.
	// Source: https://developers.cloudflare.com/cache/concepts/default-cache-behavior/
	StaticFileExtensions = []string{
		"7z", "avi", "avif", "apk", "bin", "bmp", "bz2", "class", "css",
		"csv", "doc", "docx", "dmg", "ejs", "eot", "eps", "exe", "flac",
		"gif", "gz", "ico", "iso", "jar", "jpg", "jpeg", "js", "mid",
		"midi", "mkv", "mp3", "mp4", "ogg", "otf", "pdf", "pict", "pls",
		"png", "ppt", "pptx", "ps", "rar", "svg", "svgz", "swf", "tar",
		"tif", "tiff", "ttf", "webm", "webp", "woff", "woff2", "xls", "xlsx",
		"zip", "zst",

		// Custom extensions we also want to block.
		"html", "php", "cgi", "pl",
	}
)

// OptGroup choices for the subject drop-down.
type OptGroup struct {
	Header  string
	Options []Option
}

// Option for select boxes.
type Option struct {
	Value string
	Label string
}

// ChecklistOption for checkbox-lists.
type ChecklistOption struct {
	Value string
	Label string
	Help  string
}

// NotificationOptout field values (stored in user ProfileField table)
const (
	NotificationOptOutFriendPhotos          = "notif_optout_friends_photos"
	NotificationOptOutPrivatePhotos         = "notif_optout_private_photos"
	NotificationOptOutExplicitPhotos        = "notif_optout_explicit_photos"
	NotificationOptOutLikes                 = "notif_optout_likes"
	NotificationOptOutComments              = "notif_optout_comments"
	NotificationOptOutSubscriptions         = "notif_optout_subscriptions"
	NotificationOptOutFriendRequestAccepted = "notif_optout_friend_request_accepted"
	NotificationOptOutPrivateGrant          = "notif_optout_private_grant"

	// Web Push Notifications
	PushNotificationOptOutMessage = "notif_optout_push_messages"
	PushNotificationOptOutFriends = "notif_optout_push_friends"
)

// Notification opt-outs (stored in ProfileField table)
var NotificationOptOutFields = []string{
	NotificationOptOutFriendPhotos,
	NotificationOptOutPrivatePhotos,
	NotificationOptOutExplicitPhotos,
	NotificationOptOutLikes,
	NotificationOptOutComments,
	NotificationOptOutSubscriptions,
	NotificationOptOutFriendRequestAccepted,
	NotificationOptOutPrivateGrant,
}

// Push Notification opt-outs (stored in ProfileField table)
var PushNotificationOptOutFields = []string{
	PushNotificationOptOutMessage,
	PushNotificationOptOutFriends,
}
