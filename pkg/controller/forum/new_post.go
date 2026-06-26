package forum

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/log"
	"github.com/cuvou/gosocial/pkg/markdown"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/photo"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/spam"
	"github.com/cuvou/gosocial/pkg/templates"
)

// NewPost view.
func NewPost() http.HandlerFunc {
	tmpl := templates.Must("forum/new_post.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Query params.
		var (
			fragment       = r.FormValue("to")                     // forum to (new post)
			toThreadID     = r.FormValue("thread")                 // add reply to a thread ID
			quoteCommentID = r.FormValue("quote")                  // add reply to thread while quoting a comment
			editCommentID  = r.FormValue("edit")                   // edit your comment
			intent         = r.FormValue("intent")                 // preview or submit
			photoIntent    = r.FormValue("photo_intent")           // upload, remove photo attachment
			photoID        = r.FormValue("photo_id")               // existing CommentPhoto ID
			title          = r.FormValue("title")                  // for new forum post only
			message        = r.PostFormValue("message")            // comment body
			isPinned       = r.PostFormValue("pinned") == "true"   // owners or admins only
			isExplicit     = r.PostFormValue("explicit") == "true" // for thread only
			isNoReply      = r.PostFormValue("noreply") == "true"  // for thread only
			isDelete       = r.FormValue("delete") == "true"       // delete comment (along with edit=$id)
			forum          *models.Forum
			thread         *models.Thread  // if replying to a thread
			comment        *models.Comment // if editing a comment

			// If we are modifying a comment (post) and it's the OG post of the
			// thread, we show and accept the thread settings to be updated as
			// well (pinned, explicit, noreply)
			isOriginalComment bool

			// If neither the forum nor thread are explicit, show a hint to the user not to
			// share an explicit photo in their reply.
			explicitPhotoAllowed bool

			// Polls
			pollOptions        = []string{}
			pollExpires        = 3
			pollMultipleChoice = r.FormValue("poll_multiple_choice") == "true"
			isPoll             bool

			// Attached photo object.
			commentPhoto *models.CommentPhoto

			// Note: comments should have only one photo. In case multiple got uploaded in the past,
			// they'll be collected here so they can all be deleted on a new upload.
			allPhotos []*models.CommentPhoto

			// URL to redirect to, typically the thread view.
			nextURL = fmt.Sprintf("/forum/thread/%s", toThreadID)
		)

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// Look up the forum itself.
		if found, err := models.ForumByFragment(fragment); err != nil {
			session.FlashError(w, r, "Couldn't post to forum %s: not found.", fragment)
			templates.Redirect(w, "/forum")
			return
		} else {
			forum = found
		}

		// Are we manipulating a reply to an existing thread?
		if len(toThreadID) > 0 {
			if i, err := strconv.Atoi(toThreadID); err == nil {
				if found, err := models.GetThread(uint64(i)); err != nil {
					session.FlashError(w, r, "Couldn't find that thread ID!")
					templates.Redirect(w, fmt.Sprintf("/f/%s", forum.Fragment))
					return
				} else {
					thread = found
				}
			}
		}

		// Would an explicit photo attachment be allowed?
		if forum.Explicit || (thread != nil && thread.Explicit) {
			explicitPhotoAllowed = true
		}

		// If the current user can moderate the forum thread, e.g. edit or delete posts.
		// Admins can edit always, user owners of forums can only delete.
		var canModerate = currentUser.HasAdminScope(config.ScopeForumModerator) ||
			(forum.OwnerID == currentUser.ID && isDelete)

		// Does the comment have an existing Photo ID?
		if len(photoID) > 0 {
			if i, err := strconv.Atoi(photoID); err == nil {
				if found, err := models.GetCommentPhoto(uint64(i)); err != nil {
					session.FlashError(w, r, "Couldn't find comment photo ID #%d!", i)
					templates.Redirect(w, fmt.Sprintf("/f/%s", forum.Fragment))
					return
				} else {
					commentPhoto = found
				}
			}
		}

		// Are we pre-filling the message with a quotation of an existing comment?
		if len(quoteCommentID) > 0 {
			if i, err := strconv.Atoi(quoteCommentID); err == nil {
				if comment, err := models.GetComment(uint64(i)); err == nil {

					// Prefill the message with the @mention and quoted post.
					message = fmt.Sprintf(
						"[@%s](/go/comment?id=%d)\n\n%s\n\n",
						comment.User.Username,
						comment.ID,
						markdown.Quotify(comment.Message),
					)
				}
			}
		}

		// Is the user over their photo storage quota?
		var isOverQuota bool
		if forum.PermitPhotos {
			if ok, _, _ := models.IsOverQuota(currentUser); ok {
				isOverQuota = ok
			}
		}

		// Are we editing or deleting our comment?
		if len(editCommentID) > 0 {
			if i, err := strconv.Atoi(editCommentID); err == nil {
				if found, err := models.GetComment(uint64(i)); err == nil {
					comment = found

					// Verify that it is indeed OUR comment.
					if currentUser.ID != comment.UserID && !canModerate {
						templates.ForbiddenPage(w, r)
						return
					}

					// Initialize the form w/ the content of this message.
					if r.Method == http.MethodGet {
						message = comment.Message
					}

					// Did this comment have a picture? Load it if so.
					if photos, err := comment.GetPhotos(); err == nil && len(photos) > 0 {
						commentPhoto = photos[0]
						allPhotos = photos
					}

					// Is this the OG thread of the post?
					if thread.CommentID == comment.ID {
						isOriginalComment = true

						// Restore the checkbox option form values from thread settings.
						if r.Method == http.MethodGet {
							isPinned = thread.Pinned
							isExplicit = thread.Explicit
							isNoReply = thread.NoReply
						}
					}

					// Are we DELETING this comment?
					if isDelete {
						// Is there a photo attachment? Remove it, too.
						if commentPhoto != nil {
							for _, item := range allPhotos {
								if item.Filename != "" {
									if err := photo.Delete(item.Filename); err != nil {
										session.FlashError(w, r, "Error removing the photo from disk: %s", err)
									}
								}

								if err := item.Delete(); err != nil {
									session.FlashError(w, r, "Couldn't remove photo from DB: %s", err)
								} else {
									commentPhoto = nil
								}
							}
						}

						if err := thread.DeleteReply(comment); err != nil {
							session.FlashError(w, r, "Error deleting your post: %s", err)
						} else {
							session.Flash(w, r, "Your post has been deleted.")

							// Log the change.
							models.LogDeleted(&models.User{ID: comment.UserID}, currentUser, "comments", comment.ID, fmt.Sprintf(
								"Deleted a forum comment on thread %d forum /f/%s", thread.ID, forum.Fragment,
							), comment)
						}
						templates.Redirect(w, nextURL)
						return
					}
				} else {
					// Comment not found - show the Forbidden page anyway.
					templates.ForbiddenPage(w, r)
					return
				}
			} else {
				templates.NotFoundPage(w, r)
				return
			}
		}

		// Submitting the form.
		if r.Method == http.MethodPost {
			// Look for spammy links to video sites or things.
			if err := spam.DetectSpamLinks(title + message); err != nil {
				session.FlashError(w, r, "%s", err.Error())
				if thread != nil {
					templates.Redirect(w, nextURL)
				} else if forum != nil {
					templates.Redirect(w, fmt.Sprintf("/f/%s", forum.Fragment))
				} else {
					templates.Redirect(w, "/forum")
				}
				return
			}

			// Polls: parse form parameters into a neat list of answers.
			pollExpires, _ = strconv.Atoi(r.FormValue("poll_expires"))
			var distinctPollChoices = map[string]interface{}{}
			for i := 0; i < config.PollMaxAnswers; i++ {
				if value := r.FormValue(fmt.Sprintf("answer%d", i)); value != "" {
					pollOptions = append(pollOptions, value)
					isPoll = len(pollOptions) >= 2

					// Make sure every option is distinct!
					if _, ok := distinctPollChoices[value]; ok {
						session.FlashError(w, r, "Your poll options must all be unique! Duplicate option '%s' seen in your post!", value)
						intent = "preview" // do not continue to submit
					}
					distinctPollChoices[value] = nil
				}
			}

			// If only one poll option, warn about it.
			if len(pollOptions) == 1 {
				session.FlashError(w, r, "Your poll should have at least two choices.")
				intent = "preview" // do not continue to submit
			}

			// Is a photo coming along?
			if forum.PermitPhotos {
				// Removing or replacing?
				if photoIntent == "remove" || photoIntent == "upload" || photoIntent == "replace" {
					// Remove the attached photos.
					if commentPhoto != nil {
						for _, item := range allPhotos {
							photo.Delete(item.Filename)
							if err := item.Delete(); err != nil {
								session.FlashError(w, r, "Couldn't remove photo from DB: %s", err)
							} else {
								commentPhoto = nil
							}
						}
					}
				}

				// Uploading a new picture?
				if photoIntent == "upload" || photoIntent == "replace" {
					log.Info("Receiving a photo upload for forum post")

					// Is the user already over their quota?
					if isOverQuota {
						session.FlashError(w, r, "You are currently over your allowed media quota and can not upload a new image.")
						templates.Redirect(w, nextURL)
						return
					}

					// Get their file upload.
					file, header, err := r.FormFile("file")
					if err != nil {
						session.FlashError(w, r, "Error receiving your file: %s", err)
						templates.Redirect(w, r.URL.Path)
						return
					}

					// Read the file contents.
					log.Debug("Receiving uploaded file (%d bytes): %s", header.Size, header.Filename)
					var buf bytes.Buffer
					io.Copy(&buf, file)

					filename, _, err := photo.UploadPhoto(photo.UploadConfig{
						Extension: filepath.Ext(header.Filename),
						Data:      buf.Bytes(),
					})
					if err != nil {
						session.FlashError(w, r, "Error in UploadPhoto: %s", err)
						templates.Redirect(w, r.URL.Path)
						return
					}

					// Create the PhotoComment. If we don't have a Comment ID yet, let it be empty.
					ptmpl := models.CommentPhoto{
						Filename: filename,
						UserID:   currentUser.ID,
					}
					if comment != nil {
						ptmpl.CommentID = comment.ID
					}

					// Get the filesize.
					if stat, err := os.Stat(photo.DiskPath(filename)); err == nil {
						ptmpl.Filesize = stat.Size()
					}

					// Create it in DB!
					p, err := models.CreateCommentPhoto(ptmpl)
					if err != nil {
						session.FlashError(w, r, "Couldn't create CommentPhoto in DB: %s", err)
					} else {
						log.Info("New photo! %+v", p)
					}

					commentPhoto = p
				}
			}

			// Default intent is preview unless told to submit.
			if intent == "submit" {
				// A message OR a photo is required.
				if forum.PermitPhotos && message == "" && commentPhoto == nil {
					session.FlashError(w, r, "A message OR photo is required for this post.")
					if thread != nil {
						templates.Redirect(w, nextURL)
					} else if forum != nil {
						templates.Redirect(w, fmt.Sprintf("/f/%s", forum.Fragment))
					} else {
						templates.Redirect(w, "/forum")
					}
					return
				} else if !forum.PermitPhotos && message == "" {
					session.FlashError(w, r, "A message is required for this post.")
					if thread != nil {
						templates.Redirect(w, nextURL)
					} else if forum != nil {
						templates.Redirect(w, fmt.Sprintf("/f/%s", forum.Fragment))
					} else {
						templates.Redirect(w, "/forum")
					}
					return
				}

				// Are we modifying an existing comment?
				if comment != nil {
					comment.Message = message

					// Can we update the thread props?
					if isOriginalComment {
						thread.Title = title
						thread.Pinned = isPinned
						thread.Explicit = isExplicit
						thread.NoReply = isNoReply
						if err := thread.Save(); err != nil {
							session.FlashError(w, r, "Couldn't save thread properties: %s", err)
						}
					}

					if err := comment.Save(); err != nil {
						session.FlashError(w, r, "Couldn't save comment: %s", err)
					} else {
						session.Flash(w, r, "Comment updated!")

						// Log the change.
						models.LogUpdated(&models.User{ID: comment.UserID}, currentUser, "comments", comment.ID, fmt.Sprintf(
							"Edited their comment on thread %d (in /f/%s):\n\n%s",
							thread.ID,
							forum.Fragment,
							message,
						), nil)
					}

					// Redirect to the comment.
					templates.Redirect(w, fmt.Sprintf("/go/comment?id=%d", comment.ID))
					return
				}

				// Are we replying to an existing thread?
				if thread != nil {
					if reply, err := thread.Reply(currentUser, message); err != nil {
						session.FlashError(w, r, "Couldn't add reply to thread: %s", err)
					} else {
						session.Flash(w, r, "Reply added to the thread!")

						// Log the change.
						models.LogCreated(currentUser, "comments", reply.ID, fmt.Sprintf(
							"Commented on thread %d:\n\n%s",
							thread.ID,
							message,
						))

						// If we're attaching a photo, link it to this reply CommentID.
						if commentPhoto != nil {
							commentPhoto.CommentID = reply.ID
							if err := commentPhoto.Save(); err != nil {
								log.Error("Couldn't save forum reply CommentPhoto.CommentID: %s", err)
							}
						}

						// Notify watchers about this new post. Filter by blocked user IDs.
						for _, userID := range models.FilterBlockingUserIDs(currentUser, models.GetSubscribers("threads", thread.ID)) {
							if userID == currentUser.ID {
								continue
							}

							notif := &models.Notification{
								UserID:    userID,
								AboutUser: *currentUser,
								Type:      models.NotificationAlsoPosted,
								TableName: "threads",
								TableID:   thread.ID,
								Message:   message,
								Link:      fmt.Sprintf("/go/comment?id=%d", reply.ID),
							}
							if err := models.CreateNotification(notif); err != nil {
								log.Error("Couldn't create thread reply notification for subscriber %d: %s", userID, err)
							}
						}

						// Subscribe the current user to further responses on this thread.
						if !currentUser.NotificationOptOut(config.NotificationOptOutSubscriptions) {
							if _, err := models.SubscribeTo(currentUser, "threads", thread.ID); err != nil {
								log.Error("Couldn't subscribe user %d to forum thread %d: %s", currentUser.ID, thread.ID, err)
							}
						}

						// Redirect the poster to see their new comment.
						templates.Redirect(w, fmt.Sprintf("/go/comment?id=%d", reply.ID))
						return
					}

					// Called on the error case that the post couldn't be created -
					// probably should not happen.
					templates.Redirect(w, nextURL)
					return
				}

				// Create a new thread?
				if thread, err := models.CreateThread(
					currentUser,
					forum.ID,
					title,
					message,
					isPinned,
					isExplicit,
					isNoReply,
				); err != nil {
					session.FlashError(w, r, "Couldn't create thread: %s", err)
				} else {
					session.Flash(w, r, "Thread created!")

					// The correct URL to the new thread.
					nextURL = fmt.Sprintf("/forum/thread/%d", thread.ID)

					// If we're attaching a photo, link it to this CommentID.
					if commentPhoto != nil {
						commentPhoto.CommentID = thread.CommentID
						if err := commentPhoto.Save(); err != nil {
							log.Error("Couldn't save forum post CommentPhoto.CommentID: %s", err)
						}
					}

					// Are we attaching a poll to this new thread?
					if isPoll {
						log.Info("It's a Poll! Options: %+v", pollOptions)
						poll := models.CreatePoll(pollOptions, pollExpires)
						poll.MultipleChoice = pollMultipleChoice
						if err := poll.Save(); err != nil {
							session.FlashError(w, r, "Error creating poll: %s", err)
						}

						// Attach it to this thread.
						thread.PollID = &poll.ID
						if err := thread.Save(); err != nil {
							log.Error("Couldn't save PollID onto thread! %s", err)
						}
					}

					// Subscribe the current user to responses on this thread.
					if !currentUser.NotificationOptOut(config.NotificationOptOutSubscriptions) {
						if _, err := models.SubscribeTo(currentUser, "threads", thread.ID); err != nil {
							log.Error("Couldn't subscribe user %d to forum thread %d: %s", currentUser.ID, thread.ID, err)
						}
					}

					// Log the change.
					models.LogCreated(currentUser, "threads", thread.ID, fmt.Sprintf(
						"Started a new forum thread on forum /f/%s (%s)\n\n"+
							"* Has poll? %v\n"+
							"* Title: %s\n\n%s",
						forum.Fragment,
						forum.Title,
						isPoll,
						thread.Title,
						message,
					))

					templates.Redirect(w, nextURL)
					return
				}
			}
		}

		var vars = map[string]interface{}{
			"Forum":                forum,
			"Thread":               thread,
			"Intent":               intent,
			"PostTitle":            title,
			"EditCommentID":        editCommentID,
			"EditThreadSettings":   isOriginalComment,
			"ExplicitPhotoAllowed": explicitPhotoAllowed,
			"Message":              message,

			// Thread settings (for editing the original comment esp.)
			"IsPinned":   isPinned,
			"IsExplicit": isExplicit,
			"IsNoReply":  isNoReply,

			// Polls
			"PollOptions":        pollOptions,
			"PollExpires":        pollExpires,
			"PollExpiresOptions": config.PollExpires,

			// Attached photo.
			"CommentPhoto": commentPhoto,

			"IsOverQuota": isOverQuota,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
