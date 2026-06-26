package admin

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Feedback controller (/admin/feedback)
func Feedback() http.HandlerFunc {
	tmpl := templates.Must("admin/feedback.html")

	// Whitelist for ordering options.
	var sortWhitelist = []string{
		"created_at desc",
		"created_at asc",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Query params.
		var (
			acknowledged = r.FormValue("acknowledged") == "true"
			intent       = r.FormValue("intent")
			visit        = r.FormValue("visit") == "true"   // visit the linked table ID
			profile      = r.FormValue("profile") == "true" // visit associated user profile
			verdict      = r.FormValue("verdict")
			fb           *models.Feedback

			// Search filters.
			searchQuery = r.FormValue("q")
			search      = models.ParseSearchString(searchQuery)
			subject     = r.FormValue("subject")
			sort        = r.FormValue("sort")
			sortOK      bool
		)

		// Sort options.
		for _, v := range sortWhitelist {
			if sort == v {
				sortOK = true
				break
			}
		}
		if !sortOK {
			sort = sortWhitelist[0]
		}

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get your current user: %s", err)
		}

		// Working on a target message?
		if idStr := r.FormValue("id"); idStr != "" {
			if idInt, err := strconv.Atoi(idStr); err != nil {
				session.FlashError(w, r, "Couldn't parse id param: %s", err)
			} else {
				fb, err = models.GetFeedback(uint64(idInt))
				if err != nil {
					session.FlashError(w, r, "Couldn't load feedback message %d: %s", idInt, err)
				}
			}
		}

		// Are we visiting a linked resource (via TableID)?
		if fb != nil && fb.TableID > 0 && visit {
			// New (Oct 17 '24): feedbacks may carry an AboutUserID, e.g. for photos in case the reported
			// photo is removed then the associated owner of the photo is still carried in the report.
			var aboutUser *models.User
			if fb.AboutUserID > 0 {
				if user, err := models.GetUser(fb.AboutUserID); err == nil {
					aboutUser = user
				}
			}

			switch fb.TableName {
			case "users":
				user, err := models.GetUser(fb.TableID)
				if err != nil {
					session.FlashError(w, r, "Couldn't visit user %d: %s", fb.TableID, err)
				} else {
					templates.Redirect(w, "/u/"+user.Username)
					return
				}
			case "photos":
				pic, err := models.GetPhoto(fb.TableID)
				if err != nil {
					// If there was an About User, visit their profile page instead.
					if aboutUser != nil {
						session.FlashError(w, r, "The photo #%d was deleted, visiting the owner's profile page instead.", fb.TableID)
						templates.Redirect(w, "/u/"+aboutUser.Username)
						return
					}

					session.FlashError(w, r, "Couldn't get photo %d: %s", fb.TableID, err)
				} else {
					// Going to the user's profile page?
					if profile {

						// Going forward: the aboutUser will be populated, this is for legacy reports.
						if aboutUser == nil {
							if user, err := models.GetUser(pic.UserID); err == nil {
								aboutUser = user
							} else {
								session.FlashError(w, r, "Couldn't visit user %d: %s", fb.TableID, err)
							}
						}

						if aboutUser != nil {
							templates.Redirect(w, "/u/"+aboutUser.Username)
							return
						}
					}

					// Direct link to the photo.
					templates.Redirect(w, fmt.Sprintf("/photo/view?id=%d", fb.TableID))
					return
				}
			case "messages":
				// To read this message we will redirect to the new Message Reader view.
				message, err := models.GetMessage(fb.TableID)
				if err != nil {
					session.FlashError(w, r, "Couldn't get that message ID %d: %s", fb.TableID, err)
				} else {
					templates.Redirect(w, fmt.Sprintf("/admin/message-reader?user_id=%d&partner_id=%d&view=website", fb.UserID, message.SourceUserID))
				}
			case "comments":
				// Redirect to the comment redirector.
				templates.Redirect(w, fmt.Sprintf("/go/comment?id=%d", fb.TableID))
				return
			case "blogs":
				// Blog redirector.
				templates.Redirect(w, fmt.Sprintf("/go/blog?id=%d", fb.TableID))
				return
			case "forums":
				// Get this forum.
				forum, err := models.GetForum(fb.TableID)
				if err != nil {
					session.FlashError(w, r, "Couldn't get comment ID %d: %s", fb.TableID, err)
				} else {
					templates.Redirect(w, fmt.Sprintf("/f/%s", forum.Fragment))
					return
				}
			default:
				session.FlashError(w, r, "Couldn't visit TableID %s/%d: not a supported TableName", fb.TableName, fb.TableID)
			}
		}

		// Are we (un)acknowledging a message?
		if r.Method == http.MethodPost {
			if fb == nil {
				session.FlashError(w, r, "Missing feedback ID for this POST!")
			} else {
				switch verdict {
				case "acknowledge":
					fb.Acknowledged = true
					if err := fb.Save(); err != nil {
						session.FlashError(w, r, "Couldn't save message: %s", err)
					} else {
						session.Flash(w, r, "Message acknowledged!")
					}
				case "unacknowledge":
					fb.Acknowledged = false
					if err := fb.Save(); err != nil {
						session.FlashError(w, r, "Couldn't save message: %s", err)
					} else {
						session.Flash(w, r, "Message acknowledged!")
					}
				default:
					session.FlashError(w, r, "Unsupported verdict: %s", verdict)
				}
			}

			templates.Redirect(w, r.URL.Path)
			return
		}

		// Get the feedback.
		pager := &models.Pagination{
			Page:    1,
			PerPage: config.PageSizeAdminFeedback,
			Sort:    sort,
		}
		pager.ParsePage(r)
		page, err := models.PaginateFeedback(acknowledged, intent, subject, search, pager)
		if err != nil {
			session.FlashError(w, r, "Couldn't load feedback from DB: %s", err)
		}

		// Map user IDs.
		var (
			userIDs  = []uint64{}
			photoIDs = []uint64{}
		)
		for _, p := range page {
			if p.UserID > 0 {
				userIDs = append(userIDs, p.UserID)
			}

			if p.TableName == "photos" && p.TableID > 0 {
				photoIDs = append(photoIDs, p.TableID)
			}
		}
		userMap, err := models.MapUsers(currentUser, userIDs)
		if err != nil {
			session.FlashError(w, r, "Couldn't map user IDs: %s", err)
		}

		// Map photo IDs.
		photoMap, err := models.MapPhotos(photoIDs)
		if err != nil {
			session.FlashError(w, r, "Couldn't map photo IDs: %s", err)
		}

		var vars = map[string]interface{}{
			// Filter settings.
			"DistinctSubjects": models.DistinctFeedbackSubjects(),
			"SearchTerm":       searchQuery,
			"Subject":          subject,
			"Sort":             sort,

			"Intent":       intent,
			"Acknowledged": acknowledged,
			"Feedback":     page,
			"UserMap":      userMap,
			"PhotoMap":     photoMap,
			"Pager":        pager,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
