package config

// Number of page buttons to show on a pager. Default shows page buttons
// 1 thru N (e.g., 1 thru 8) or w/ your page number in the middle surrounded
// by its neighboring pages.
const (
	PagerButtonLimit = 6 // only even numbers make a difference
)

// Pagination sizes per page.
var (
	PageSizeMemberSearch           = 60
	PageSizeFriends                = 12
	PageSizeFollowers              = PageSizeFriends
	PageSizePrivateShareFriends    = PageSizeFriends
	PageSizeBlockList              = PageSizeFriends
	PageSizeMuteList               = PageSizeFriends
	PageSizePrivatePhotoGrantees   = 12
	PageSizeAdminFeedback          = 20
	PageSizeAdminFeedbackNotesPage = 10 // feedback on User Notes page
	PageSizeAdminInboxThread       = 50 // Message Reader view.
	PageSizeAdminIPInsights        = 50 // IP Address Insights.
	PageSizeChangeLog              = 20
	PageSizeAdminUserNotes         = 10 // other users' notes
	PageSizeSiteGallery            = 16
	PageSizeUserGallery            = PageSizeSiteGallery
	PageSizePhotoAlbumPreview      = 6 // max albums to show as preview
	PageSizePhotoAlbumList         = 12
	PageSizeMyMedia                = 24
	PageSizeInboxList              = 20 // sidebar list
	PageSizeInboxThread            = 10 // conversation view
	PageSizeBrowseForums           = 20
	PageSizeForums                 = 100 // TODO: for main category index view
	PageSizeMyListForums           = 20  // "My List" pager on forum home (categories) page.
	PageSizeThreadList             = 20  // 20 threads per board, 20 posts per thread
	PageSizeForumAdmin             = 20
	PageSizeNotificationsQuery     = 100 // query from database
	PageSizeNotificationsShow      = 50  // show on page (after 'like' compression)
	PageSizeLikeList               = 12  // number of likes to show in popup modal
	PageSizeLoginSessions          = 50
	PageSizeUsernameTypeAhead      = 10
)
