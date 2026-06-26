// Package notification handles front-end formatting and consolidation for on-site notifications.
package notification

import (
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/models"
)

// Notification holds the data and verbiage for a single UI notification row.
type Notification struct {
	Request *http.Request

	// Database Notification model IDs for this row.
	IDs    []uint64
	Models []*models.Notification

	// 'NEW!' badge on front-end if any Notification is unread.
	Read bool

	// Primary 'About User' to display with this row (e.g. who commented on your picture,
	// or who most recently liked something with many recent likes).
	AboutUser *models.User

	// Type of notification ('like', 'comment', etc.)
	Type models.NotificationType

	// Subject text and link for the notification.
	// SubjectVerb string // e.g. "commented on your"
	SubjectName  string // e.g. liked your "photo" or "profile page" or "comment"
	SubjectClass string // class for the <a> on the subject, e.g. 'has-text-private'
	SubjectVowel bool   // subject name begins with a vowel, e.g. "an event" not "a event"
	Link         string // e.g. URL to the photo or comment.

	// Markdown formatted message body.
	Message     string
	HideMessage bool
	FullMessage bool // don't truncate the message on frontend.

	Timestamp time.Time

	// Attachments to the notification.
	Photo  *models.Photo
	Thread *models.Thread
	// Comment?

	// Compressed notification fields.
	OtherUnread    int
	OtherUsernames []string // multiple users "like" one thing
	OtherCount     int      // one user "likes" multiple things
}

// FromModels initializes a set of UI Notifications from a page of DB models.
func FromModels(currentUser *models.User, r *http.Request, rows []*models.Notification) []*Notification {
	var result = []*Notification{}

	// Map attached models for these notifications.
	var notifMap = models.NotificationMap{}
	if models.DB != nil {
		notifMap = models.MapNotifications(currentUser, rows)
	}

	for _, row := range rows {
		body := notifMap.Get(row.ID)

		var n = &Notification{
			Request: r,

			IDs:    []uint64{row.ID},
			Models: []*models.Notification{row},

			Read:      row.Read,
			AboutUser: &row.AboutUser,
			Type:      row.Type,
			Message:   row.Message,
			Link:      row.Link,
			Timestamp: row.CreatedAt,

			Photo:  body.Photo,
			Thread: body.Thread,
		}

		// Subject details per type or table name.
		switch n.Type {
		case models.NotificationAlsoPosted:
			n.SubjectName = "a forum thread"
		case models.NotificationPrivatePhoto:
			n.SubjectName = "private photos"
		case models.NotificationNewPhoto:
			n.SubjectName = "photo"
			if n.Photo != nil && n.Photo.Visibility == models.PhotoPrivate {
				n.SubjectName = "private photo"
				n.SubjectClass = "has-text-private"
			}
		case models.NotificationFollow,
			models.NotificationFollowBack:
			n.SubjectName = "followed"
		default:
			switch row.TableName {
			case "photos":
				n.SubjectName = "photo"
			case "videos":
				n.SubjectName = "video"
			case "users":
				n.SubjectName = "profile page"
			case "comments":
				n.SubjectName = "comment"
			case "blogs":
				n.SubjectName = "blog post"
			case "travel_plans":
				n.SubjectName = "travel plan"
			case "events":
				n.SubjectName = "event"
			case "supporter_plans":
				n.SubjectName = ""
			default:
				n.SubjectName = row.TableName
			}
		}

		// Grammar: is the subject name beginning with a vowel?
		if n.SubjectName != "" {
			switch n.SubjectName[0] {
			case 'a', 'e', 'i', 'o', 'u':
				n.SubjectVowel = true
			}
		}

		result = append(result, n)
	}

	return result
}

// ID returns the notification ID(s), comma separated if multiple.
func (n Notification) ID() string {
	var IDs []string
	for _, id := range n.IDs {
		IDs = append(IDs, fmt.Sprintf("%d", id))
	}
	return strings.Join(IDs, ",")
}

/*
SummaryLine returns the HTML snippet for the header of a notification.

Examples look like:

- $Username liked your [photo].
- $Username and 12 others liked your [comment].
- $Username commented on your [photo].
- $Username also commented on a [photo] that you replied to:
- $Username replied to [a forum thread] that you follow:
- $Username accepted your friend request!
*/
func (n Notification) SummaryLine() template.HTML {
	var (
		parts       = []string{}
		suffix      string
		punctuation = ":"
	)

	// "Username"... prefix.
	switch n.Type {
	case models.NotificationForumModerator:
		// Don't show (it will be the current user's username)
	default:
		var punctuation string
		if len(n.OtherUsernames) > 1 {
			punctuation = ","
		}

		if n.AboutUser != nil {
			parts = append(parts, fmt.Sprintf(
				`<a href="/u/%s"><strong>%s</strong></a>%s`,
				n.AboutUser.Username,
				n.AboutUser.Username,
				punctuation,
			))
		}

		// Other usernames? (compressed notification)
		if len(n.OtherUsernames) == 1 {
			parts = append(parts, fmt.Sprintf(
				`and <a href="/u/%s"><strong>%s</strong></a>`,
				n.OtherUsernames[0],
				n.OtherUsernames[0],
			))
		} else if len(n.OtherUsernames) >= 2 {
			plural := "s"
			if len(n.OtherUsernames)-1 == 1 {
				plural = ""
			}
			parts = append(parts, fmt.Sprintf(
				`<a href="/u/%s"><strong>%s</strong></a>, and %d other%s`,
				n.OtherUsernames[0],
				n.OtherUsernames[0],
				len(n.OtherUsernames)-1,
				plural,
			))
		}
	}

	// "Liked your..."
	var subjectPlural string
	switch n.Type {
	case models.NotificationLike:
		// Did one user like multiple of your things?
		if n.OtherCount > 0 {
			parts = append(parts, fmt.Sprintf("liked %d of your", n.OtherCount+1))
			subjectPlural = "s"
		} else {
			parts = append(parts, "liked your")
		}
	case models.NotificationFriendApproved:
		parts = append(parts, "accepted your friend request")
		punctuation = "."
	case models.NotificationFollow:
		parts = append(parts, "has")
		suffix = "you"
		punctuation = "."
	case models.NotificationFollowBack:
		parts = append(parts, "has")
		suffix = "you back"
		punctuation = "."
	case models.NotificationComment:
		parts = append(parts, "commented on your")
	case models.NotificationAlsoCommented:
		an := "a"
		if n.SubjectVowel {
			an = "an"
		}
		parts = append(parts, "also commented on "+an)
		suffix = "that you replied to"
	case models.NotificationAlsoPosted:
		parts = append(parts, "replied to")
		suffix = "that you follow"
	case models.NotificationTaggedUser:
		parts = append(parts, "has tagged you in their")
	case models.NotificationPrivatePhoto:
		parts = append(parts, "has granted you access to see their")
	case models.NotificationNewPhoto:
		parts = append(parts, "has uploaded a new")
	case models.NotificationForumModerator:
		parts = append(parts, `You have been appointed as a <strong class="has-text-success">moderator</strong> for the forum`)
	default:
		parts = append(parts, string(n.Type))
	}

	// "photo" (link)
	if n.Link != "" && n.SubjectName != "" {
		class := ""
		if n.SubjectClass != "" {
			class = fmt.Sprintf(` class="%s"`, n.SubjectClass)
		}
		parts = append(parts, fmt.Sprintf(`<a href="%s"%s>%s</a>`, n.Link, class, n.SubjectName+subjectPlural))
	} else if n.SubjectName != "" {
		if n.SubjectClass != "" {
			parts = append(parts, fmt.Sprintf(`<span class="%s">%s</span>`, n.SubjectClass, n.SubjectName+subjectPlural))
		} else {
			parts = append(parts, n.SubjectName+subjectPlural)
		}
	}

	if suffix != "" {
		parts = append(parts, suffix)
	}

	return template.HTML(fmt.Sprintf(
		"<span>%s%s</span>",
		strings.Join(parts, " "),
		punctuation,
	))
}

// FooterLine returns the HTML snippet for the footer, e.g. "See all comments"
func (n Notification) FooterLine() template.HTML {
	var parts = []string{}

	// Attached forum thread?
	if n.Thread != nil {
		link := n.Link
		if link == "" {
			link = fmt.Sprintf("/forum/thread?id=%d", n.Thread.ID)
		}
		parts = append(parts, fmt.Sprintf(`
			<div class="block">
				On thread: <a href="%s">%s</a>
			</div>
		`, link, n.Thread.Title))
	}

	// Links that may go under photos (comments, explicit, or just its caption)
	if n.Photo != nil {
		switch n.Type {
		case models.NotificationComment, models.NotificationAlsoCommented:
			parts = append(parts, fmt.Sprintf(`
				<div class="block">
					<div class="is-size-7 pt-1">
						<span class="icon"><i class="fa fa-arrow-right"></i></span>
						<a href="%s">See all comments</a>
					</div>
				</div>
			`, n.Link))
		default:
			caption := "No caption."
			if n.Photo.Caption != "" {
				caption = n.Photo.Caption
			}
			parts = append(parts, fmt.Sprintf(`<em>%s</em>`, caption))
		}
	}

	return template.HTML(strings.Join(parts, " "))
}

// UnsubscribeLine returns the HTML snippet for a contextual 'unsubscribe' link.
func (n Notification) UnsubscribeLine() template.HTML {
	var (
		show    bool
		link    string
		confirm string
		class   string
		title   string
		verb    string

		username  string
		tableName string
		tableID   uint64
		nextURL   string
	)

	if n.Request != nil {
		nextURL = url.QueryEscape(n.Request.URL.String())
	}

	if len(n.Models) > 0 {
		tableName = n.Models[0].TableName
		tableID = n.Models[0].TableID
	}

	if n.AboutUser != nil {
		username = n.AboutUser.Username
	}

	switch n.Type {
	case models.NotificationAlsoPosted, models.NotificationAlsoCommented:
		show = true
		link = fmt.Sprintf(
			"/comments/subscription?table_name=%s&table_id=%d&next=%s&subscribe=false",
			tableName, tableID, nextURL,
		)
		confirm = "Do you want to TURN OFF notifications about this comment thread?"
		class = "has-text-warning is-small gosocial-mute-notification-link"
		title = "Turn off notifications about this thread"
		verb = "Mute this thread"
	case models.NotificationNewPhoto:
		return template.HTML(fmt.Sprintf(`
			<small>
				<a href="/unfollow/confirm?username=%s&next=/me"
					class="has-text-warning is-small"
					title="Unfollow %s">
					<i class="fa fa-microphone-slash mr-1"></i>
					Unfollow %s
				</a>
			</small>
		`, username, username, username))
	}

	if !show {
		return template.HTML("")
	}

	return template.HTML(fmt.Sprintf(`
		<small>
			<a href="#"
				data-link="%s"
				data-confirm="%s"
				class="%s"
				title="%s">
				<i class="fa fa-microphone-slash mr-1"></i>
				%s
			</a>
		</small>
	`, link, confirm, class, title, verb))
}

// Icon returns the FontAwesome CSS icon class for this notification.
func (n Notification) Icon() string {
	switch n.Type {
	case models.NotificationLike:
		return "fa fa-heart has-text-danger"
	case models.NotificationFriendApproved:
		return "fa fa-user-group has-text-success"
	case models.NotificationFollow,
		models.NotificationFollowBack:
		return "fa fa-star has-text-success"
	case models.NotificationComment, models.NotificationAlsoCommented:
		return "fa fa-comment has-text-success"
	case models.NotificationAlsoPosted:
		return "fa fa-comments has-text-success"
	case models.NotificationTaggedUser:
		return "fa fa-tag has-text-success"
	case models.NotificationPrivatePhoto:
		return "fa fa-unlock has-text-private"
	case models.NotificationNewPhoto:
		icon := "fa"

		var (
			isPrivate  bool
			isExplicit bool
			baseIcon   string
		)
		if n.Photo != nil {
			baseIcon = "fa-image"
			isPrivate = n.Photo.Visibility == models.PhotoPrivate
			isExplicit = n.Photo.Explicit
		} else {
			baseIcon = "fa-image has-text-warning"
		}

		if isPrivate {
			icon += " fa-eye"
			if !isExplicit {
				icon += " has-text-private"
			}
		} else {
			icon += " " + baseIcon
			if !isExplicit {
				icon += " has-text-link"
			}
		}

		if isExplicit {
			icon += " has-text-danger"
		}

		return icon
	case models.NotificationForumModerator:
		return "fa fa-user-tie has-text-success"
	default:
		return "fa fa-exclamation-triangle has-text-warning"
	}
}
