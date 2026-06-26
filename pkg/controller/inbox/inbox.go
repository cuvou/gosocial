package inbox

import (
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Inbox is where users receive direct messages.
func Inbox() http.HandlerFunc {
	tmpl := templates.Must("inbox/inbox.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Message ID in path? (/messages/read/{id} endpoint)
		var msgId int
		if idStr := r.PathValue("id"); idStr != "" {
			msgId, _ = strconv.Atoi(idStr)
		}

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Unexpected error: could not get currentUser.")
			templates.Redirect(w, "/")
			return
		}

		// What view are we looking at? Threads (default), Inbox, or Outbox?
		var box = r.FormValue("box")
		if box != "inbox" && box != "sent" && box != "all" {
			box = "threads"
		}

		// Sorting unread messages first?
		var (
			searchTerm = r.FormValue("search")
			search     = models.ParseSearchString(searchTerm)
			sortUnread = r.FormValue("unread") != "false"
			sort       = "read asc, created_at desc"
		)
		if !sortUnread {
			sort = "created_at desc"
		}

		// Are we reading a specific message?
		var (
			viewThread    []*models.Message
			threadPager   *models.Pagination
			composeToUser *models.User
		)
		if msgId > 0 {
			if msg, err := models.GetMessage(uint64(msgId)); err != nil {
				session.FlashError(w, r, "Message not found.")
				templates.Redirect(w, "/messages")
				return
			} else {
				// We must be a party to this thread.
				if msg.SourceUserID != currentUser.ID && msg.TargetUserID != currentUser.ID {
					templates.ForbiddenPage(w, r)
					return
				}

				// Find the other party in this thread.
				var senderUserID = msg.SourceUserID
				if senderUserID == currentUser.ID {
					senderUserID = msg.TargetUserID
				}

				// Look up the sender's username to compose a response to them.
				sender, err := models.GetUser(senderUserID)
				if err != nil {
					session.FlashError(w, r, "Couldn't get sender of that message: %s", err)
				}
				composeToUser = sender

				// Get the full chat thread (paginated).
				threadPager = &models.Pagination{
					PerPage: config.PageSizeInboxThread,
					Sort:    "created_at desc",
				}
				threadPager.ParsePage(r)
				thread, err := models.GetMessageThread(msg.SourceUserID, msg.TargetUserID, threadPager)
				if err != nil {
					session.FlashError(w, r, "Couldn't get chat history: %s", err)
				}

				viewThread = thread

				// Mark all these messages as read if the recipient sees them.
				for _, m := range viewThread {
					if m.TargetUserID == currentUser.ID && !m.Read {
						m.Read = true
						if err := m.Save(); err != nil {
							session.FlashError(w, r, "Couldn't mark message as read: %s", err)
						}
					}
				}
			}
		}

		// Get the inbox list of messages.
		var messages []*models.Message
		pager := &models.Pagination{
			Page:    1,
			PerPage: config.PageSizeInboxList,
			Sort:    sort,
		}
		if viewThread == nil {
			// On the main inbox view, ?page= params page thru the message list, not a thread.
			pager.ParsePage(r)
		}

		// Viewing the threads, or a specific inbox/sent box?
		if box == "threads" {
			if result, err := models.GetMessageThreads(currentUser, search, pager); err != nil {
				session.FlashError(w, r, "Couldn't get your messages from DB: %s", err)
			} else {
				messages = result
			}
		} else {
			if result, err := models.GetMessages(currentUser, box == "sent", box == "all", search, pager); err != nil {
				session.FlashError(w, r, "Couldn't get your messages from DB: %s", err)
			} else {
				messages = result
			}
		}

		// How many unreads?
		unread, err := models.CountUnreadMessages(currentUser)
		if err != nil {
			session.FlashError(w, r, "Couldn't get your unread message count from DB: %s", err)
		}

		// Map sender data on these messages.
		var userIDs = []uint64{}
		for _, m := range messages {
			userIDs = append(userIDs, m.SourceUserID, m.TargetUserID)
		}
		for _, m := range viewThread {
			userIDs = append(userIDs, m.SourceUserID, m.TargetUserID)
		}
		userMap, err := models.MapUsers(currentUser, userIDs)
		if err != nil {
			session.FlashError(w, r, "Couldn't map users: %s", err)
		}

		var vars = map[string]interface{}{
			"Messages":     messages,
			"UserMap":      userMap,
			"Unread":       unread,
			"IsSortUnread": sortUnread,
			"SearchString": searchTerm,
			"Pager":        pager,
			"Box":          box,
			"ViewThread":   viewThread, // nil on inbox page
			"ThreadPager":  threadPager,
			"ReplyTo":      composeToUser,
			"MessageID":    msgId,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
