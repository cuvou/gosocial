package models

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/cuvou/gosocial/pkg/log"
	"gorm.io/gorm"
)

// AdminGroup table gives a name to a set of admin permission scopes.
type AdminGroup struct {
	ID        uint64 `gorm:"primaryKey"`
	Name      string `gorm:"index"`
	CreatedAt time.Time
	UpdatedAt time.Time
	Scopes    []AdminScope `gorm:"foreignKey:GroupID"`
	Users     []*User      `gorm:"many2many:admin_group_users;"`
}

// AdminScope table maps admin group IDs to named scopes (which may contain wildcards).
type AdminScope struct {
	ID      uint64 `gorm:"primaryKey"`
	GroupID uint64 `gorm:"index"`
	Scope   string
}

// Preload related tables for the AdminGroup.
func (g *AdminGroup) Preload() *gorm.DB {
	return DB.Preload("Scopes").Preload("Users")
}

// HasAdminScope returns whether the user is an admin and has the requested scope.
func (u *User) HasAdminScope(scope string) bool {
	if !u.IsAdmin {
		return false
	}

	for _, group := range u.AdminGroups {
		for _, compare := range group.Scopes {
			// Scope has a wildcard?
			if strings.ContainsRune(compare.Scope, '*') {
				re := regexp.MustCompile(
					fmt.Sprintf(`^%s$`, strings.ReplaceAll(compare.Scope, "*", ".+?")),
				)
				if res := re.FindStringSubmatch(scope); len(res) > 0 {
					log.Debug("Regexp scope '%s' matched requested '%s'", compare.Scope, scope)
					return true
				}
			} else if compare.Scope == scope {
				return true
			}
		}
	}

	return false
}

// ListAdminGroups returns the list of admin group (names) the user belongs to.
func (u *User) ListAdminGroups() []string {
	var names = []string{}
	for _, group := range u.AdminGroups {
		names = append(names, group.Name)
	}
	sort.Strings(names)
	return names
}

// ListAdminScopes returns the list of admin group (names) the user belongs to.
func (u *User) ListAdminScopes() []string {
	var (
		names    = []string{}
		distinct = map[string]interface{}{}
	)
	for _, group := range u.AdminGroups {
		for _, scope := range group.Scopes {
			if _, ok := distinct[scope.Scope]; ok {
				continue
			}
			distinct[scope.Scope] = nil
			names = append(names, scope.Scope)
		}
	}
	sort.Strings(names)
	return names
}

// CreateAdminGroup inserts a new admin group.
func CreateAdminGroup(name string, scopes []string) (*AdminGroup, error) {
	// Verify the name is unique.
	if _, err := FindAdminGroup(name); err == nil {
		return nil, fmt.Errorf("that group name already exists: %s", err)
	}

	g := &AdminGroup{
		Name:   name,
		Scopes: []AdminScope{},
	}
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope != "" {
			g.Scopes = append(g.Scopes, AdminScope{
				Scope: scope,
			})
		}
	}

	result := DB.Create(g)
	return g, result.Error
}

// FindAdminGroup by name.
func FindAdminGroup(name string) (*AdminGroup, error) {
	if name == "" {
		return nil, errors.New("username is required")
	}

	g := &AdminGroup{}
	result := g.Preload().Where("name = ?", name).Limit(1).First(g)
	return g, result.Error
}

// ListAdminGroups returns all admin groups.
func ListAdminGroups() ([]*AdminGroup, error) {
	var groups = []*AdminGroup{}

	result := (&AdminGroup{}).Preload().Order("name asc").Find(&groups)
	return groups, result.Error
}

// ListAdminUsers returns all admin user accounts.
func ListAdminUsers() ([]*User, error) {
	var users = []*User{}

	result := (&User{}).Preload().Where("is_admin is true").Order("username asc").Find(&users)
	return users, result.Error
}

// ScopesString returns the scopes as a newline separated string.
func (g *AdminGroup) ScopesString() string {
	var scopes = []string{}
	for _, scope := range g.Scopes {
		scopes = append(scopes, scope.Scope)
	}
	return strings.Join(scopes, "\n")
}

// ReplaceScopes sets new scopes for the admin group.
func (g *AdminGroup) ReplaceScopes(scopes []string) error {
	// Delete the original scopes.
	var replace = []AdminScope{}
	for _, scope := range scopes {
		scope = strings.TrimSpace(scope)
		if scope != "" {
			replace = append(replace, AdminScope{
				Scope: scope,
			})
		}
	}

	err := DB.Model(g).Association("Scopes").Replace(replace)

	// Cleanup orphaned scopes.
	if result := DB.Where("group_id IS NULL").Delete(&AdminScope{}); result.Error != nil {
		log.Error("AdminGroup.ReplaceScopes: cleanup orphaned scopes: %s", result.Error)
	}

	return err
}

// HasAdmin returns whether the given username is a member of the group.
func (g *AdminGroup) HasAdmin(username string) bool {
	for _, user := range g.Users {
		if user.Username == username {
			return true
		}
	}
	return false
}

// ReplaceUsers easily adds or removes admin users from a group.
//
// Post the full list of usernames who should be in the group, and it will add or
// remove users as needed and return which users were changed.
func (g *AdminGroup) ReplaceUsers(usernames []string) (added, removed []string, err error) {
	added = []string{}
	removed = []string{}

	// Map who is currently in the group.
	var currentUsernames = map[string]interface{}{}
	var allUsernames = []string{}
	for _, user := range g.Users {
		currentUsernames[user.Username] = nil
		allUsernames = append(allUsernames, user.Username)
	}

	// Map the incoming username list.
	var incomingUsernames = map[string]interface{}{}

	// Who needs added?
	for _, username := range usernames {
		incomingUsernames[username] = nil
		allUsernames = append(allUsernames, username)
		if _, ok := currentUsernames[username]; !ok {
			added = append(added, username)
		}
	}

	// Who needs removed?
	for username := range currentUsernames {
		if _, ok := incomingUsernames[username]; !ok {
			removed = append(removed, username)
		}
	}

	log.Info("AdminGroup(%s).ReplaceUsers: complete list %s (adding: %s) (removing: %s)",
		g.Name,
		usernames,
		added,
		removed,
	)

	// Select all affected users from the DB.
	usermap, err := MapUsersByUsername(allUsernames)
	if err != nil {
		return
	}

	// Do the needful.
	var replace = []*User{}
	for _, username := range usernames {
		if user, ok := usermap[username]; ok {
			replace = append(replace, user)
		}
	}

	err = DB.Model(g).Association("Users").Replace(replace)

	// Clean up orphaned admin_group_users.
	if result := DB.Exec(`DELETE FROM admin_group_users WHERE admin_group_id IS NULL`); result.Error != nil {
		log.Error("AdminGroup.ReplaceUsers: cleanup orphaned users: %s", result.Error)
	}

	return
}

// AddUser puts the given user into the admin group.
func (g *AdminGroup) AddUser(user *User) error {
	if user.AdminGroups == nil {
		user.AdminGroups = []*AdminGroup{}
	}

	// Already exists?
	for _, group := range user.AdminGroups {
		if group.ID == g.ID {
			return nil
		}
	}

	// Insert them.
	user.AdminGroups = append(user.AdminGroups, g)
	return user.Save()
}

// RemoveUser removes a user from the admin group.
func (g *AdminGroup) RemoveUser(user *User) error {
	if user.AdminGroups == nil {
		return errors.New("user had no admin groups")
	}

	return DB.Model(user).Association("AdminGroups").Delete(g)
}

// Save group.
func (g *AdminGroup) Save() error {
	result := DB.Save(g)
	return result.Error
}

// Delete the AdminGroup.
func (g *AdminGroup) Delete() error {
	// Remove scopes.
	if result := DB.Where("group_id = ?", g.ID).Delete(&AdminScope{}); result.Error != nil {
		return fmt.Errorf("can't remove scopes for group ID %d: %s", g.ID, result.Error)
	}

	// Remove users.
	if _, _, err := g.ReplaceUsers([]string{}); err != nil {
		return fmt.Errorf("can't remove users for group ID %d: %s", g.ID, err)
	}

	return DB.Delete(g).Error
}
