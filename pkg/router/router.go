// Package router configures web routes.
package router

import (
	"net/http"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/controller/account"
	"github.com/cuvou/gosocial/pkg/controller/admin"
	"github.com/cuvou/gosocial/pkg/controller/api"
	"github.com/cuvou/gosocial/pkg/controller/api/barertc"
	"github.com/cuvou/gosocial/pkg/controller/block"
	"github.com/cuvou/gosocial/pkg/controller/chat"
	"github.com/cuvou/gosocial/pkg/controller/comment"
	"github.com/cuvou/gosocial/pkg/controller/follows"
	"github.com/cuvou/gosocial/pkg/controller/forum"
	"github.com/cuvou/gosocial/pkg/controller/friend"
	"github.com/cuvou/gosocial/pkg/controller/htmx"
	"github.com/cuvou/gosocial/pkg/controller/inbox"
	"github.com/cuvou/gosocial/pkg/controller/index"
	"github.com/cuvou/gosocial/pkg/controller/photo"
	"github.com/cuvou/gosocial/pkg/controller/poll"
	"github.com/cuvou/gosocial/pkg/controller/settings"
	"github.com/cuvou/gosocial/pkg/middleware"
	nst "github.com/cuvou/gosocial/pkg/templates"
	"github.com/cuvou/gosocial/pkg/webpush"
)

func New() http.Handler {
	mux := http.NewServeMux()

	// Register controller endpoints.
	mux.HandleFunc("/", index.Create())
	mux.HandleFunc("GET /favicon.ico", index.Favicon())
	mux.HandleFunc("GET /manifest.json", index.Manifest())
	mux.HandleFunc("GET /robots.txt", index.Robots())
	mux.HandleFunc("GET /sw.js", index.ServiceWorker())
	mux.HandleFunc("GET /about", index.StaticTemplate("about.html")())
	mux.HandleFunc("GET /features", index.StaticTemplate("features.html")())
	mux.HandleFunc("GET /insights", index.Demographics())
	mux.HandleFunc("GET /tos", index.StaticTemplate("tos.html")())
	mux.HandleFunc("GET /privacy", index.StaticTemplate("privacy.html")())
	mux.HandleFunc("/contact", index.Contact())
	mux.HandleFunc("/login", account.Login())
	mux.HandleFunc("GET /logout", account.Logout())
	mux.Handle("/signup", middleware.GeoGate(account.Signup()))
	mux.HandleFunc("/forgot-password", account.ForgotPassword())
	mux.HandleFunc("GET /settings/confirm-email", settings.ConfirmEmailChange())
	mux.HandleFunc("GET /markdown", index.StaticTemplate("markdown.html")())
	mux.HandleFunc("GET /test/geo-gate", index.StaticTemplate("errors/geo_gate.html")())

	// Login Required. Pages that non-certified users can access.
	mux.Handle("/me", middleware.LoginRequired(account.Dashboard()))
	mux.Handle("/settings", middleware.LoginRequired(index.StaticTemplate("settings/index.html")()))
	mux.Handle("/settings/profile", middleware.LoginRequired(settings.Profile()))
	mux.Handle("POST /settings/status", middleware.LoginRequired(settings.StatusMessage()))
	mux.Handle("/settings/look", middleware.LoginRequired(settings.Look()))
	mux.Handle("/settings/prefs", middleware.LoginRequired(settings.Prefs()))
	mux.Handle("/settings/location", middleware.LoginRequired(settings.Location()))
	mux.Handle("/settings/privacy", middleware.LoginRequired(settings.Privacy()))
	mux.Handle("/settings/notifications", middleware.LoginRequired(settings.Notifications()))
	mux.Handle("/settings/account", middleware.LoginRequired(settings.Account()))
	mux.Handle("/settings/sessions", middleware.LoginRequired(settings.LoginSessions()))
	mux.Handle("GET /settings/deactivate", middleware.LoginRequired(index.StaticTemplate("settings/deactivate.html")()))
	mux.Handle("/settings/essays", middleware.LoginRequired(account.EditEssays()))
	mux.Handle("/settings/age-gate", middleware.LoginRequired(account.AgeGate()))
	mux.Handle("/settings/security-checkup", middleware.LoginRequired(account.SecurityCheckup()))
	mux.Handle("/settings/theme", middleware.LoginRequired(account.WebsiteTheme()))
	mux.Handle("GET /settings/safe-mode", middleware.LoginRequired(settings.SafeMode()))
	mux.Handle("/account/two-factor/setup", middleware.LoginRequired(account.Setup2FA()))
	mux.Handle("/account/delete", middleware.LoginRequired(account.Delete()))
	mux.Handle("/account/deactivate", middleware.LoginRequired(account.Deactivate()))
	mux.Handle("GET /account/reactivate", middleware.LoginRequired(account.Reactivate()))
	mux.Handle("GET /u/{username}", account.Profile()) // public access OK
	mux.Handle("GET /u/{username}/friends", middleware.LoginRequired(account.UserFriends()))
	mux.Handle("GET /u/{username}/photos", middleware.LoginRequired(photo.UserPhotos()))
	mux.Handle("GET /u/{username}/albums", middleware.LoginRequired(photo.UserAlbums()))
	mux.Handle("GET /u/{username}/album/{album}", middleware.LoginRequired(photo.UserPhotos()))
	mux.Handle("/photo/upload", middleware.LoginRequired(photo.Upload()))
	mux.Handle("GET /photo/view", middleware.LoginRequired(photo.View()))
	mux.Handle("/photo/edit", middleware.LoginRequired(photo.Edit()))
	mux.Handle("/photo/delete", middleware.LoginRequired(photo.Delete()))
	mux.Handle("/photo/batch-edit", middleware.LoginRequired(photo.BatchEdit()))
	mux.Handle("GET /photo/private", middleware.LoginRequired(photo.Private()))
	mux.Handle("/photo/private/share", middleware.LoginRequired(photo.Share()))
	mux.Handle("GET /photo/media", middleware.LoginRequired(photo.MyMedia()))
	mux.Handle("GET /messages", middleware.LoginRequired(inbox.Inbox()))
	mux.Handle("GET /messages/read/{id}", middleware.LoginRequired(inbox.Inbox()))
	mux.Handle("/messages/compose", middleware.LoginRequired(inbox.Compose()))
	mux.Handle("/messages/delete", middleware.LoginRequired(inbox.Delete()))
	mux.Handle("GET /friends", middleware.LoginRequired(friend.Friends()))
	mux.Handle("/friends/add", middleware.LoginRequired(friend.AddFriend()))
	mux.Handle("GET /followers", middleware.LoginRequired(follows.Followers()))
	mux.Handle("GET /following", middleware.LoginRequired(follows.Following()))
	mux.Handle("POST /following/add", middleware.LoginRequired(follows.Follow()))
	mux.Handle("POST /followers/edit", middleware.LoginRequired(follows.EditFollower()))
	mux.Handle("POST /followers/batch-remove", middleware.LoginRequired(follows.BatchRemove()))
	mux.Handle("GET /unfollow/confirm", middleware.LoginRequired(follows.ConfirmUnfollow()))
	mux.Handle("POST /users/block", middleware.LoginRequired(block.BlockUser()))
	mux.Handle("GET /users/blocked", middleware.LoginRequired(block.Blocked()))
	mux.Handle("GET /users/blocklist/add", middleware.LoginRequired(block.AddUser()))
	mux.Handle("/comments", middleware.LoginRequired(comment.PostComment()))
	mux.Handle("/comments/batch-edit", middleware.LoginRequired(comment.BatchEdit()))
	mux.Handle("GET /comments/subscription", middleware.LoginRequired(comment.Subscription()))
	mux.Handle("GET /admin/unimpersonate", middleware.LoginRequired(admin.Unimpersonate()))
	mux.Handle("GET /admin/transparency", middleware.LoginRequired(admin.Transparency()))
	mux.Handle("GET /admin/transparency/{username}", middleware.LoginRequired(admin.Transparency()))
	mux.Handle("/users/tag", middleware.LoginRequired(account.TagUser()))

	// Certification Required. Pages that only full (verified) members can access.
	mux.Handle("GET /photo/gallery", middleware.LoginRequired(photo.SiteGallery()))
	mux.Handle("GET /members", middleware.LoginRequired(account.Search()))
	mux.Handle("/chat", middleware.LoginRequired(chat.Landing()))
	mux.Handle("GET /forum", middleware.LoginRequired(forum.Landing()))
	mux.Handle("/forum/post", middleware.LoginRequired(forum.NewPost()))
	mux.Handle("GET /forum/thread/{id}", middleware.LoginRequired(forum.Thread()))
	mux.Handle("POST /forum/thread/{id}/moderate", middleware.LoginRequired(forum.ModerateThread()))
	mux.Handle("GET /forum/explore", middleware.LoginRequired(forum.Explore()))
	mux.Handle("GET /forum/newest", middleware.LoginRequired(forum.Newest()))
	mux.Handle("GET /forum/search", middleware.LoginRequired(forum.Search()))
	mux.Handle("POST /forum/subscribe", middleware.LoginRequired(forum.Subscribe()))
	mux.Handle("GET /f/{fragment}", middleware.LoginRequired(forum.Forum()))
	mux.Handle("POST /poll/vote", middleware.LoginRequired(poll.Vote()))
	mux.Handle("/forum/admin", middleware.LoginRequired(forum.Manage()))
	mux.Handle("/forum/admin/edit", middleware.LoginRequired(forum.AddEdit()))
	mux.Handle("/forum/admin/delete", middleware.LoginRequired(forum.Delete()))
	mux.Handle("/forum/admin/moderator", middleware.LoginRequired(forum.ManageModerators()))

	// Admin endpoints.
	mux.Handle("GET /admin", middleware.AdminRequired("", admin.Dashboard()))
	mux.Handle("/admin/scopes", middleware.AdminRequired("", admin.Scopes()))
	mux.Handle("/admin/feedback", middleware.AdminRequired(config.ScopeFeedbackAndReports, admin.Feedback()))
	mux.Handle("/admin/user-action", middleware.AdminRequired("", admin.UserActions()))
	mux.Handle("/admin/maintenance", middleware.AdminRequired(config.ScopeMaintenance, admin.Maintenance()))
	mux.Handle("/admin/add-user", middleware.AdminRequired(config.ScopeUserCreate, admin.AddUser()))
	mux.Handle("/admin/photo/mark-explicit", middleware.AdminRequired("", admin.MarkPhotoExplicit()))
	mux.Handle("GET /admin/changelog", middleware.AdminRequired(config.ScopeChangeLog, admin.ChangeLog()))
	mux.Handle("/forum/admin/move-thread", middleware.AdminRequired(config.ScopeForumModerator, forum.MoveThread()))
	mux.Handle("/admin/bulk-message", middleware.AdminRequired(config.ScopeBulkMessage, admin.BulkMessage()))
	mux.Handle("/admin/message-reader", middleware.AdminRequired(config.ScopeUserMessages, admin.MessageReader()))
	mux.Handle("/admin/ip-addresses", middleware.AdminRequired(config.ScopeUserInsight, admin.IPInsights()))

	// JSON API endpoints.
	mux.HandleFunc("GET /v1/version", api.Version())
	mux.HandleFunc("GET /v1/debug", api.Debug())
	mux.HandleFunc("GET /v1/auth/static", api.PhotoSignAuth())
	mux.HandleFunc("GET /v1/users/me", api.LoginOK())
	mux.HandleFunc("POST /v1/users/check-username", api.UsernameCheck())
	mux.HandleFunc("GET /v1/web-push/vapid-public-key", webpush.VAPIDPublicKey)
	mux.Handle("GET /v1/panic", middleware.AdminRequired("", api.TestPanic()))
	mux.Handle("POST /v1/web-push/register", middleware.LoginRequired(webpush.Register()))
	mux.Handle("GET /v1/web-push/unregister", middleware.LoginRequired(webpush.UnregisterAll()))
	mux.Handle("POST /v1/likes", middleware.LoginRequired(api.Likes()))
	mux.Handle("GET /v1/likes/users", middleware.LoginRequired(api.WhoLikes()))
	mux.Handle("POST /v1/photo/{photo_id}/view", middleware.LoginRequired(api.ViewPhoto()))
	mux.Handle("POST /v1/notifications/read", middleware.LoginRequired(api.ReadNotification()))
	mux.Handle("POST /v1/notifications/delete", middleware.LoginRequired(api.ClearNotification()))
	mux.Handle("GET /v1/world-cities", middleware.LoginRequired(api.WorldCities()))
	mux.Handle("GET /v1/time-zones", middleware.LoginRequired(api.TimeZones()))
	mux.Handle("GET /v1/users/username", middleware.LoginRequired(api.SearchUsernames()))
	mux.Handle("POST /v1/location/pretty-flag", middleware.LoginRequired(api.WorldCitiesPretty()))
	mux.Handle("POST /v1/barertc/report", barertc.Report())
	mux.Handle("POST /v1/barertc/profile", barertc.Profile())
	mux.Handle("POST /v1/barertc/friends", barertc.Friends())
	mux.Handle("POST /v1/markdown", middleware.LoginRequired(api.MarkdownPreview()))

	// HTMX endpoints.
	mux.Handle("GET /htmx/user/profile/activity", middleware.LoginRequired(htmx.UserProfileActivityCard()))
	mux.Handle("GET /htmx/comments", htmx.CommentThread())
	mux.Handle("GET /htmx/friend-picker", middleware.LoginRequired(htmx.FriendPicker()))
	mux.Handle("GET /htmx/photo/footer", htmx.LoadGalleryEmbeds())

	// Redirect endpoints.
	mux.Handle("GET /go/comment", middleware.LoginRequired(comment.GoToComment()))
	mux.Handle("GET /go/comment-photo", middleware.LoginRequired(comment.GoToCommentPhoto()))
	mux.Handle("GET /go/photo", middleware.LoginRequired(comment.GoToPhoto()))
	mux.Handle("GET /go/tagged", middleware.LoginRequired(comment.GoToTaggedUser()))
	mux.Handle("GET /go/profile-photo/{username}", middleware.LoginRequired(photo.GoToProfilePhoto()))

	// Static files.
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(config.StaticPath))))

	// Legacy route redirects (Go 1.22 path parameters update)
	mux.Handle("GET /friends/u/{s}", nst.RedirectRoute("/u/%s/friends"))
	mux.Handle("GET /photo/u/{s}", nst.RedirectRoute("/u/%s/photos"))
	mux.Handle("GET /notes/u/{s}", nst.RedirectRoute("/u/%s/notes"))

	// Global middlewares.
	withCSRF := middleware.CSRF(mux)
	withSession := middleware.Session(withCSRF)
	withRecovery := middleware.Recovery(withSession)
	withLogger := middleware.Logging(withRecovery)
	return withLogger
}
