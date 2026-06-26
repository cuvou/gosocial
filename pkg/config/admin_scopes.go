package config

// All available admin scopes
const (

	// Common (Base) Admin permissions.
	ScopeAdminBase = "admin.base"

	// Social moderation over the chat and forums.
	// - Chat: have operator controls in the chat room
	// - Forum: ability to edit and delete user posts
	// - Photo: omniscient view of all gallery photos, can edit/delete photos
	ScopeChatModerator  = "social.moderator.chat"
	ScopeForumModerator = "social.moderator.forum"
	ScopePhotoModerator = "social.moderator.photo"

	// Website administration
	// - Forum: ability to manage available forums
	// - Scopes: ability to manage admin groups & scopes
	// - Maintenance mode
	ScopeForumAdmin      = "admin.forum.manage"
	ScopeAdminScopeAdmin = "admin.scope.manage"
	ScopeMaintenance     = "admin.maintenance"

	// User account admin
	// - Impersonate: ability to log in as a user account
	// - Ban: ability to ban/unban users
	// - Delete: ability to delete user accounts
	ScopeUserCreate      = "admin.user.create"
	ScopeUserInsight     = "admin.user.insights"
	ScopeUserImpersonate = "admin.user.impersonate"
	ScopeUserBan         = "admin.user.ban"
	ScopeUserPassword    = "admin.user.password"
	ScopeUserDelete      = "admin.user.delete"
	ScopeUserPromote     = "admin.user.promote"
	ScopeManage2FA       = "admin.user.2fa"
	ScopeUserMessages    = "admin.user.messages"

	// Other admin views
	ScopeFeedbackAndReports = "admin.feedback"
	ScopeChangeLog          = "admin.changelog"
	ScopeUserNotes          = "admin.user.notes"
	ScopeBulkMessage        = "admin.bulk-message"

	// Admins with this scope can not be blocked by users.
	ScopeUnblockable = "admin.unblockable"

	// The global wildcard scope gets all available permissions.
	ScopeSuperuser = "*"
)

// Friendly description for each scope.
var AdminScopeDescriptions = map[string]string{
	ScopeAdminBase:          "Common features for all admins: access to the Admin Forum, ability to share User Notes with other admins.",
	ScopeChatModerator:      "Have operator controls in the chat room (can mark cameras as explicit, or kick/ban people from chat).",
	ScopeForumModerator:     "Ability to moderate the forum (edit or delete posts).",
	ScopePhotoModerator:     "Ability to moderate photo galleries (can see all private or friends-only photos, and edit their properties, admin labels and approval status).",
	ScopeForumAdmin:         "Ability to manage forums themselves (add or remove forums, edit their properties).",
	ScopeAdminScopeAdmin:    "Ability to manage admin permissions for other admin accounts.",
	ScopeMaintenance:        "Ability to activate maintenance mode functions of the website (turn features on or off, disable signups or logins, etc.)",
	ScopeUserCreate:         "Ability to manually create a new user account, bypassing the signup page.",
	ScopeUserInsight:        "Ability to see admin insights about a user profile (e.g. their block lists and who blocks them).",
	ScopeUserImpersonate:    "Ability to log in as any user account. Note: this action is logged and notifies the admin team when it happens, and admins must write a reason. This is used only to e.g. diagnose customer support issues or investigate a reported Direct Message conversation they had (DMs can not be read otherwise).",
	ScopeUserBan:            "Ability to ban or unban user accounts.",
	ScopeUserPassword:       "Ability to reset a user's password on their behalf.",
	ScopeUserDelete:         "Ability to fully delete user accounts on their behalf.",
	ScopeUserPromote:        "Ability to add or remove the admin status flag on a user profile.",
	ScopeManage2FA:          "Ability to manage Two-Factor Auth settings and help a user regain access to their account.",
	ScopeUserMessages:       "Ability to access user messages to investigate a report or flagged chat. Note: the admin must enter a reason why they are accessing messages and it is logged as a report for the admin team to see.",
	ScopeFeedbackAndReports: "Ability to see admin reports and user feedback.",
	ScopeChangeLog:          "Ability to see website change logs.",
	ScopeUserNotes:          "Ability to see all user notes. Note: Admins by default may share their own notes with other admins, but can not see notes written by users about each other.",
	ScopeBulkMessage:        "Ability to broadcast a Direct Message to multiple people at once.",
	ScopeUnblockable:        "This admin can not be added to user block lists.",
	ScopeSuperuser:          "This admin has access to ALL admin features on the website.",
}

// Number of expected scopes for unit test and validation.
const QuantityAdminScopes = 21

// The specially named Superusers group.
const AdminGroupSuperusers = "Superusers"

// ListAdminScopes returns the listing of all available admin scopes.
func ListAdminScopes() []string {
	return []string{
		ScopeAdminBase,
		ScopeChatModerator,
		ScopeForumModerator,
		ScopePhotoModerator,
		ScopeForumAdmin,
		ScopeAdminScopeAdmin,
		ScopeMaintenance,
		ScopeUserCreate,
		ScopeUserInsight,
		ScopeUserImpersonate,
		ScopeUserBan,
		ScopeUserPassword,
		ScopeUserDelete,
		ScopeUserPromote,
		ScopeManage2FA,
		ScopeUserMessages,
		ScopeFeedbackAndReports,
		ScopeChangeLog,
		ScopeUserNotes,
		ScopeBulkMessage,
		ScopeUnblockable,
	}
}

func AdminScopeDescription(scope string) string {
	return AdminScopeDescriptions[scope]
}
