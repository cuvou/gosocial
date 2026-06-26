package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/cuvou/gosocial/pkg/config"
	"github.com/cuvou/gosocial/pkg/models"
	"github.com/cuvou/gosocial/pkg/session"
	"github.com/cuvou/gosocial/pkg/templates"
)

// Scopes controller (/admin/scopes)
func Scopes() http.HandlerFunc {
	tmpl := templates.Must("admin/scopes.html")
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Query parameters.
		var (
			intent         = r.FormValue("intent")
			editGroupIDStr = r.FormValue("id")
		)

		currentUser, err := session.CurrentUser(r)
		if err != nil {
			session.FlashError(w, r, "Couldn't get your current user: %s", err)
		}

		_ = currentUser

		// List all of the admin users, groups & scopes.
		adminUsers, err := models.ListAdminUsers()
		if err != nil {
			session.FlashError(w, r, "ListAdminUsers: %s", err)
		}
		adminGroups, err := models.ListAdminGroups()
		if err != nil {
			session.FlashError(w, r, "ListAdminGroups: %s", err)
		}

		// Does the Superusers group exist yet?
		var needSuperuserInit = true
		for _, group := range adminGroups {
			if group.Name == config.AdminGroupSuperusers {
				needSuperuserInit = false
				break
			}
		}

		// Validate that the Edit Group ID exists.
		var (
			editGroupID int
			editGroup   = &models.AdminGroup{}
		)
		if editGroupIDStr != "" {
			groupID, err := strconv.Atoi(editGroupIDStr)
			if err != nil {
				session.FlashError(w, r, "Group ID is not a valid integer")
				templates.Redirect(w, r.URL.Path)
				return
			}

			var found bool
			for _, group := range adminGroups {
				if group.ID == uint64(groupID) {
					editGroup = group
					found = true
					break
				}
			}

			if !found && groupID > 0 {
				session.FlashError(w, r, "Group ID not found.")
				templates.Redirect(w, r.URL.Path)
				return
			}

			editGroupID = groupID
		}

		// POST event handlers.
		if r.Method == http.MethodPost {
			// Scope check.
			if !currentUser.HasAdminScope(config.ScopeAdminScopeAdmin) && intent != "init-superusers" {
				session.FlashError(w, r, "Missing admin scope: %s", config.ScopeAdminScopeAdmin)
				templates.Redirect(w, r.URL.Path)
				return
			}

			switch intent {
			case "init-superusers":
				// Initialize the Superusers group, if it does not already exist.
				if !needSuperuserInit {
					session.FlashError(w, r, "Could not initialize the Superusers group: it already exists.")
					break
				}

				group, err := models.CreateAdminGroup(config.AdminGroupSuperusers, []string{"*"})
				if err != nil {
					session.FlashError(w, r, "Couldn't create Superusers group: %s", err)
					break
				}

				// Add the current admin user to it.
				if err := group.AddUser(currentUser); err != nil {
					session.FlashError(w, r, "Couldn't add you to the Superusers group: %s", err)
					break
				}

				session.Flash(w, r, "The Superusers group has been initialized and you placed in it.")
			case "save":
				// Create or Save an AdminGroup.
				var (
					groupName   = r.PostFormValue("name")
					groupScopes = strings.Split(r.PostFormValue("scopes"), "\n")
					groupUsers  = r.PostForm["username"]
				)

				if editGroupID == 0 {
					// New group: easiest option.
					group, err := models.CreateAdminGroup(groupName, groupScopes)
					if err != nil {
						session.FlashError(w, r, "Couldn't create new group: %s", err)
						break
					}

					// Apply the user list to it.
					added, removed, err := group.ReplaceUsers(groupUsers)
					if err != nil {
						session.FlashError(w, r, "Couldn't save users in this group: %s", err)
						break
					}

					session.Flash(w, r, "Saved admin group with %d scopes.", len(groupScopes))
					if len(added) > 0 {
						session.Flash(w, r, "Added %s to the group.", strings.Join(added, ", "))
					}
					if len(removed) > 0 {
						session.Flash(w, r, "Removed %s from the group.", strings.Join(removed, ", "))
					}
				} else {
					// Updating the existing group.
					if err := editGroup.ReplaceScopes(groupScopes); err != nil {
						session.FlashError(w, r, "Couldn't replace scopes: %s", err)
						break
					}

					added, removed, err := editGroup.ReplaceUsers(groupUsers)
					if err != nil {
						session.FlashError(w, r, "Couldn't save users in this group: %s", err)
						break
					}

					editGroup.Name = groupName
					if err := editGroup.Save(); err != nil {
						session.FlashError(w, r, "Couldn't save group name: %s", err)
						break
					}

					session.Flash(w, r, "Saved admin group with %d scopes.", len(groupScopes))
					if len(added) > 0 {
						session.Flash(w, r, "Added %s to the group.", strings.Join(added, ", "))
					}
					if len(removed) > 0 {
						session.Flash(w, r, "Removed %s from the group.", strings.Join(removed, ", "))
					}
				}
			case "delete":
				if editGroupID == 0 {
					session.FlashError(w, r, "Can't delete group: no group ID")
				} else {
					err := editGroup.Delete()
					if err != nil {
						session.FlashError(w, r, "Couldn't delete group: %s", err)
					} else {
						session.Flash(w, r, "Group deleted!")
					}
				}
			default:
				session.FlashError(w, r, "Unsupported intent: %s", intent)
			}

			templates.Redirect(w, r.URL.Path)
			return
		}

		// Map the 2FA status of each admin.
		map2FA, err := models.MapTwoFactor(adminUsers)
		if err != nil {
			session.FlashError(w, r, "MapTwoFactor: %s", err)
		}

		var vars = map[string]interface{}{
			"Intent":            intent,
			"AdminUsers":        adminUsers,
			"TwoFactorMap":      map2FA,
			"AdminGroups":       adminGroups,
			"AdminScopes":       config.ListAdminScopes(),
			"NeedSuperuserInit": needSuperuserInit,
			"EditGroupID":       editGroupID,
			"EditGroup":         editGroup,
		}
		if err := tmpl.Execute(w, r, vars); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	})
}
