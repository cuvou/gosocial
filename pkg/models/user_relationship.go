package models

// UserRelationship fields - how a target User relates to the CurrentUser, especially
// with regards to whether the User's friends-only or private profile picture should show.
// The zero-values should fail safely: in case the UserRelationship isn't populated correctly,
// private profile pics show as private by default.
type UserRelationship struct {
	Computed         bool // check whether the SetUserRelationships function has been run
	IsFriend         bool // if true, a friends-only profile pic can show
	IsPrivateGranted bool // if true, a private profile pic can show
	IsBlocked        bool // if true, the users are blocking each other
	IsExplicitSeen   bool // if true, the viewer opts to see the explicit photo
}

// SetUserRelationships updates a set of User objects to populate their UserRelationships in
// relationship to the current user who is looking.
func SetUserRelationships(currentUser *User, users []*User) error {
	// Collect the current user's Friendships and Private Grants.
	var (
		friendIDs     = FriendIDs(currentUser.ID)
		privateGrants = PrivateGrantedUserIDs(currentUser.ID)
		blockedIDs    = BlockedUserIDs(currentUser)
	)

	// Map them for easier lookup.
	var (
		friendMap  = map[uint64]interface{}{}
		privateMap = map[uint64]interface{}{}
		blockedMap = map[uint64]interface{}{}
	)

	for _, id := range friendIDs {
		friendMap[id] = nil
	}

	for _, id := range privateGrants {
		privateMap[id] = nil
	}

	for _, id := range blockedIDs {
		blockedMap[id] = nil
	}

	// Inject the UserRelationships.
	for _, u := range users {
		u.UserRelationship.Computed = true

		if u.ID == currentUser.ID {
			// Current user - set both bools to true - you can always see your own profile pic.
			u.UserRelationship.IsFriend = true
			u.UserRelationship.IsPrivateGranted = true
			u.UserRelationship.IsExplicitSeen = true
			continue
		}

		// Explicit profile photo vs. the viewer's opt-in setting.
		if currentUser.Explicit {
			u.UserRelationship.IsExplicitSeen = true
		}

		if _, ok := friendMap[u.ID]; ok {
			u.UserRelationship.IsFriend = true
		}

		if _, ok := privateMap[u.ID]; ok {
			u.UserRelationship.IsPrivateGranted = true
		}

		if _, ok := blockedMap[u.ID]; ok {
			u.UserRelationship.IsBlocked = true
		}
	}
	return nil
}

// SetUserRelationshipsInComments takes a set of Comments and sets relationship booleans on their Users.
func SetUserRelationshipsInComments(user *User, comments []*Comment) {
	// Gather and map the users.
	var (
		users   = []*User{}
		userMap = map[uint64]*User{}
	)

	for _, c := range comments {
		users = append(users, &c.User)
		userMap[c.User.ID] = &c.User
	}

	// Inject relationships.
	SetUserRelationships(user, users)
}

// SetUserRelationshipsInThreads takes a set of Threads and sets relationship booleans on their Users.
func SetUserRelationshipsInThreads(user *User, threads []*Thread) {
	// Gather and map the thread parent comments.
	var (
		comments = []*Comment{}
		comMap   = map[uint64]*Comment{}
	)

	for _, c := range threads {
		comments = append(comments, &c.Comment)
		comMap[c.Comment.ID] = &c.Comment
	}

	// Inject relationships into those comments' users.
	SetUserRelationshipsInComments(user, comments)
}

// SetUserRelationshipsInNotifications takes a set of Notifications and sets relationship booleans on their AboutUsers.
func SetUserRelationshipsInNotifications(user *User, notifications []*Notification) {
	// Gather and map the users.
	var (
		users   = []*User{}
		userMap = map[uint64]*User{}
	)

	for _, n := range notifications {
		users = append(users, &n.AboutUser)
		userMap[n.AboutUser.ID] = &n.AboutUser
	}

	// Inject relationships.
	SetUserRelationships(user, users)
}
