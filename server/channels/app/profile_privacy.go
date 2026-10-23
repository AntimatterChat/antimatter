// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

// Profile privacy (Antimatter): which details of their profile people show to the people of each community, kept in
// their profile_privacy prop (see model/profile_privacy.go). The server enforces it wherever it sends profiles out to
// someone else (the user routes, search and autocomplete, group and thread participants, attributes, websocket
// events, push and email notifications): the details hidden are blanked, and so is the prop, so that nobody is told
// what someone hides. System admins see everything (the System Console needs it); so does the person themselves.

// profileViewer works out which details of people's profiles one person sees, loading the communities they're in
// once, the first time someone shown to them hides something.
type profileViewer struct {
	store  store.Store
	id     string
	exempt bool

	teams  map[string]bool
	loaded bool
	failed bool
}

func newProfileViewer(s store.Store, viewerID string, exempt bool) *profileViewer {
	return &profileViewer{store: s, id: viewerID, exempt: exempt}
}

func (a *App) newProfileViewer(viewerID string, exempt bool) *profileViewer {
	return newProfileViewer(a.Srv().Store(), viewerID, exempt)
}

// concerned tells whether the viewer might be denied something of user: neither themselves nor exempt, and user
// hides something somewhere.
func (v *profileViewer) concerned(user *model.User) bool {
	return user != nil && !v.exempt && user.Id != v.id && user.HasProfilePrivacy()
}

// hidden gives the details of user hidden from the viewer: those any community they share hides (or, with none in
// common, those hidden for all). When the communities can't be read, every detail hidden somewhere is (fail closed).
func (v *profileViewer) hidden(user *model.User) map[string]bool {
	if !v.concerned(user) {
		return nil
	}
	privacy, ok := user.GetProfilePrivacy()
	if !ok {
		return model.AllProfileFields()
	}
	if len(privacy) == 0 {
		return nil
	}
	if !v.loaded {
		v.loaded = true
		ids, err := v.store.Team().GetUserTeamIds(v.id, true)
		if err != nil {
			v.failed = true
		}
		v.teams = make(map[string]bool, len(ids))
		for _, id := range ids {
			v.teams[id] = true
		}
	}
	if v.failed {
		return privacy.AllHidden()
	}
	theirs, err := v.store.Team().GetUserTeamIds(user.Id, true)
	if err != nil {
		return privacy.AllHidden()
	}
	shared := make([]string, 0, len(theirs))
	for _, id := range theirs {
		if v.teams[id] {
			shared = append(shared, id)
		}
	}
	return privacy.HiddenFrom(shared)
}

// apply hides from the viewer what they may not see of users (copies the server sends out, already sanitised), and
// the profile privacy prop. It gives the ids of the users of whom something was hidden.
func (v *profileViewer) apply(users []*model.User) map[string]bool {
	var changed map[string]bool
	for _, user := range users {
		if !v.concerned(user) {
			continue
		}
		hidden := v.hidden(user)
		user.HideProfileFields(hidden)
		if len(hidden) > 0 {
			if changed == nil {
				changed = map[string]bool{}
			}
			changed[user.Id] = true
		}
	}
	return changed
}

// HideProfileDetails hides from viewerID the details of users they may not see (the copies the server sends out,
// already sanitised). asAdmin is for system admins, who see everything. It gives the ids of the users of whom
// something was hidden, for FilterHiddenSearchMatches.
func (a *App) HideProfileDetails(viewerID string, asAdmin bool, users ...*model.User) map[string]bool {
	return a.newProfileViewer(viewerID, asAdmin).apply(users)
}

// ProfileDetailsHiddenFrom gives the details of user hidden from viewerID (nil when none).
func (a *App) ProfileDetailsHiddenFrom(viewerID string, asAdmin bool, user *model.User) map[string]bool {
	return a.newProfileViewer(viewerID, asAdmin).hidden(user)
}

// FilterHiddenSearchMatches drops from the results of a search for term the users found only by details hidden from
// whoever searched (changed, as HideProfileDetails gave it): searching for a hidden name mustn't tell whose it is.
// Each word of the term must be in a detail they see (username, names, nickname, email), as the search does it.
func FilterHiddenSearchMatches(term string, users []*model.User, changed map[string]bool) []*model.User {
	if len(changed) == 0 {
		return users
	}
	words := strings.Fields(strings.ToLower(term))
	kept := make([]*model.User, 0, len(users))
	for _, user := range users {
		if !changed[user.Id] || matchesVisibleDetails(user, words) {
			kept = append(kept, user)
		}
	}
	return kept
}

func matchesVisibleDetails(user *model.User, words []string) bool {
	details := []string{
		strings.ToLower(user.Username),
		strings.ToLower(user.FirstName),
		strings.ToLower(user.LastName),
		strings.ToLower(user.Nickname),
		strings.ToLower(user.Email),
	}
	for _, word := range words {
		word = strings.TrimLeft(word, "@")
		if word == "" || word == strings.ToLower(user.Id) {
			continue
		}
		found := false
		for _, detail := range details {
			if detail != "" && strings.Contains(detail, word) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// attributesHiddenFrom tells, for each of userIDs, whether they hide their attributes from viewerID: true only for
// those who do. When the profiles can't be read, everyone's are hidden (fail closed).
func (a *App) attributesHiddenFrom(rctx request.CTX, viewerID string, exempt bool, userIDs []string) map[string]bool {
	hidden := map[string]bool{}
	if exempt || len(userIDs) == 0 {
		return hidden
	}
	users, err := a.Srv().Store().User().GetProfileByIds(rctx, userIDs, nil, true)
	if err != nil {
		for _, id := range userIDs {
			hidden[id] = id != viewerID
		}
		return hidden
	}
	viewer := a.newProfileViewer(viewerID, exempt)
	for _, user := range users {
		if viewer.hidden(user)[model.ProfileFieldAttributes] {
			hidden[user.Id] = true
		}
	}
	return hidden
}

// AttributesHiddenFromSession tells whether the session's user may not see userID's attributes (see
// attributesHiddenFrom); system admins see them.
func (a *App) AttributesHiddenFromSession(rctx request.CTX, session model.Session, userID string) bool {
	if session.UserId == userID {
		return false
	}
	exempt := a.SessionHasPermissionTo(session, model.PermissionManageSystem)
	return a.attributesHiddenFrom(rctx, session.UserId, exempt, []string{userID})[userID]
}

// sanitizeParticipants sanitises the participants of a thread as viewerID, whose thread it is, sees them.
func (a *App) sanitizeParticipants(participants []*model.User, viewerID string) {
	a.sanitizeProfiles(participants, false)
	a.HideProfileDetails(viewerID, false, participants...)
}

// seenBy gives the notification as recipient may see it: its sender and, for a group message, its members as they
// see them (see profileForRecipient). It's the notification itself when nobody in it hides anything.
func (n *PostNotification) seenBy(a *App, recipient *model.User) *PostNotification {
	// The members' names only name group messages.
	group := n.Channel != nil && n.Channel.Type == model.ChannelTypeGroup
	concerned := n.Sender != nil && n.Sender.HasProfilePrivacy()
	if group {
		for _, user := range n.ProfileMap {
			concerned = concerned || user.HasProfilePrivacy()
		}
	}
	if !concerned || recipient == nil || recipient.IsSystemAdmin() {
		return n
	}
	seen := *n
	seen.Sender = a.profileForRecipient(n.Sender, recipient)
	if group && len(n.ProfileMap) > 0 {
		seen.ProfileMap = make(map[string]*model.User, len(n.ProfileMap))
		for id, user := range n.ProfileMap {
			seen.ProfileMap[id] = a.profileForRecipient(user, recipient)
		}
	}
	return &seen
}

// profileForRecipient gives the user as recipient sees them, for what the server sends recipient itself (push and
// email notifications): a copy with what's hidden from them blanked, or the user when nothing is.
func (a *App) profileForRecipient(user, recipient *model.User) *model.User {
	if user == nil || recipient == nil || !user.HasProfilePrivacy() || user.Id == recipient.Id || recipient.IsSystemAdmin() {
		return user
	}
	copied := user.DeepCopy()
	a.newProfileViewer(recipient.Id, false).apply([]*model.User{copied})
	return copied
}
