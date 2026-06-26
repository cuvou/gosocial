package forum

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// AddEdit page.
func AddEdit() http.HandlerFunc {
	tmpl := templates.Must("forum/add_edit.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Are we editing a forum or creating a new one?
		var editID uint64
		if editStr := r.FormValue("id"); editStr != "" {
			if i, err := strconv.Atoi(editStr); err == nil {
				editID = uint64(i)
			} else {
				session.FlashError(w, r, "Edit parameter: id was not an integer")
				templates.Redirect(w, "/forum/admin")
				return
			}
		}

		// Get the current user.
		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get current user: %s", err)
			templates.Redirect(w, "/")
			return
		}

		// If editing, look up the existing forum.
		var forum *models.Forum
		if editID > 0 {
			if found, err := models.GetForum(editID); err != nil {
				session.FlashError(w, r, "Couldn't get forum: %s", err)
				templates.Redirect(w, "/forum/admin")
				return
			} else {
				// Do we have permission?
				if !found.CanEdit(currentUser) {
					templates.ForbiddenPage(w, r)
					return
				}

				forum = found
			}
		}

		// If we are over our quota for User Forums, do not allow creating a new one.
		if forum == nil && !currentUser.HasAdminScope(config.ScopeForumAdmin) {
			session.FlashError(w, r, "You do not currently have spare quota to create a new forum.")
			templates.Redirect(w, "/forum/admin")
			return
		}

		// Saving?
		if r.Method == http.MethodPost {
			var (
				title          = strings.TrimSpace(r.PostFormValue("title"))
				fragment       = strings.TrimSpace(strings.ToLower(r.PostFormValue("fragment")))
				description    = strings.TrimSpace(r.PostFormValue("description"))
				category       = strings.TrimSpace(r.PostFormValue("category"))
				isExplicit     = r.PostFormValue("explicit") == "true"
				isPrivileged   = r.PostFormValue("privileged") == "true"
				isPermitPhotos = r.PostFormValue("permit_photos") == "true"
				isPrivate      = r.PostFormValue("private") == "true"
			)

			// Sanity check admin-only settings -> default these to OFF.
			if !currentUser.HasAdminScope(config.ScopeForumAdmin) {
				isPrivileged = false
				isPrivate = false
			}

			// Were we editing an existing forum?
			if forum != nil {
				diffs := []models.FieldDiff{
					models.NewFieldDiff("Title", forum.Title, title),
					models.NewFieldDiff("Description", forum.Description, description),
					models.NewFieldDiff("Category", forum.Category, category),
					models.NewFieldDiff("Explicit", forum.Explicit, isExplicit),
					models.NewFieldDiff("PermitPhotos", forum.PermitPhotos, isPermitPhotos),
				}

				forum.Title = title
				forum.Description = description
				forum.Category = category
				forum.Explicit = isExplicit
				forum.PermitPhotos = isPermitPhotos

				// Forum Admin-only options: if the current viewer is not a forum admin, do not change these settings.
				// e.g.: the front-end checkboxes are hidden and don't want to accidentally unset these!
				if currentUser.HasAdminScope(config.ScopeForumAdmin) {
					diffs = append(diffs,
						models.NewFieldDiff("Privileged", forum.Privileged, isPrivileged),
						models.NewFieldDiff("Private", forum.Private, isPrivate),
					)
					forum.Privileged = isPrivileged
					forum.Private = isPrivate
				}

				// Save it.
				if err := forum.Save(); err == nil {
					session.Flash(w, r, "Forum has been updated!")
					templates.Redirect(w, "/forum/admin")

					// Log the change.
					models.LogUpdated(currentUser, nil, "forums", forum.ID, "Updated the forum's settings.", diffs)
					return
				} else {
					session.FlashError(w, r, "Error saving the forum: %s", err)
				}
			} else {
				// Validate the fragment. Front-end enforces the pattern so this
				// is just a sanity check.
				if m := FragmentRegexp.FindStringSubmatch(fragment); m == nil {
					session.FlashError(w, r, "The fragment format is invalid.")
					templates.Redirect(w, "/forum/admin")
					return
				}

				// Ensure the fragment is unique.
				if _, err := models.ForumByFragment(fragment); err == nil {
					session.FlashError(w, r, "The forum fragment is already in use.")
				} else {
					// Create the forum.
					forum = &models.Forum{
						Owner:        *currentUser,
						Category:     category,
						Fragment:     fragment,
						Title:        title,
						Description:  description,
						Explicit:     isExplicit,
						Privileged:   isPrivileged,
						PermitPhotos: isPermitPhotos,
						Private:      isPrivate,
					}

					if err := models.CreateForum(forum); err == nil {
						session.Flash(w, r, "The forum has been created!")
						templates.Redirect(w, "/forum/admin")

						// Log the change.
						models.LogCreated(currentUser, "forums", forum.ID, fmt.Sprintf(
							"Created a new forum.\n\n"+
								"* Category: %s\n"+
								"* Title: %s\n"+
								"* Fragment: %s\n"+
								"* Description: %s\n"+
								"* Explicit: %v\n"+
								"* Privileged: %v\n"+
								"* Photos: %v\n"+
								"* Private: %v",
							forum.Category,
							forum.Title,
							forum.Fragment,
							forum.Description,
							forum.Explicit,
							forum.Privileged,
							forum.PermitPhotos,
							forum.Private,
						))

						// If this is a Community forum, subscribe the owner to it immediately.
						if forum.Category == "" {
							models.CreateForumMembership(currentUser.ID, forum.ID)
						}

						return
					} else {
						session.FlashError(w, r, "Error creating the forum: %s", err)
					}
				}
			}
		}

		// Get the list of moderators.
		var mods []*models.User
		if forum != nil {
			mods, err = forum.GetModerators()
			if err != nil {
				session.FlashError(w, r, "Error getting moderators list: %s", err)
			}
		}

		var vars = map[string]interface{}{
			"EditID":     editID,
			"EditForum":  forum,
			"Categories": config.ForumCategories,
			"Moderators": mods,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
