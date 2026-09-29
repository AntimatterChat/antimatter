// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"net/http"
	"sort"
	"strings"

	"github.com/mattermost/ldap"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// maxGroupNesting bounds the depth of nested group resolution.
const maxGroupNesting = 20

// groupEntry is a group object read from the directory.
type groupEntry struct {
	DN          string // normalized DN
	RemoteID    string
	DisplayName string
	Members     []string // normalized DNs of the members
}

func newGroupEntry(ss *session, e *ldap.Entry, withMembers bool) *groupEntry {
	s := ss.s
	g := &groupEntry{
		DN:       normalizeDN(e.DN),
		RemoteID: attributeValue(e, s.GroupIdAttribute),
	}
	if s.GroupDisplayNameAttribute != "" {
		g.DisplayName = attributeValue(e, s.GroupDisplayNameAttribute)
	}
	if g.DisplayName == "" {
		g.DisplayName = g.RemoteID
	}
	if withMembers {
		for _, m := range ss.memberValues(e) {
			if m = strings.TrimSpace(m); m != "" {
				g.Members = append(g.Members, normalizeDN(m))
			}
		}
	}
	return g
}

func (g *groupEntry) toModel() *model.Group {
	displayName := g.DisplayName
	if len(displayName) > model.GroupDisplayNameMaxLength {
		displayName = displayName[:model.GroupDisplayNameMaxLength]
	}
	remoteID := g.RemoteID
	return &model.Group{
		DisplayName: displayName,
		RemoteId:    &remoteID,
		Source:      model.GroupSourceLdap,
	}
}

// fetchGroups returns the groups matching the filter that have a group ID.
func fetchGroups(ss *session, filter string, withMembers bool) ([]*groupEntry, error) {
	entries, err := ss.search(filter, ss.s.groupAttributes(withMembers))
	if err != nil {
		return nil, err
	}
	groups := make([]*groupEntry, 0, len(entries))
	for _, e := range entries {
		g := newGroupEntry(ss, e, withMembers)
		if g.RemoteID == "" {
			continue
		}
		groups = append(groups, g)
	}
	return groups, nil
}

// GetGroup returns the directory group whose group ID attribute is groupUID.
func (l *Ldap) GetGroup(rctx request.CTX, groupUID string) (*model.Group, *model.AppError) {
	s := l.settings()
	if s.GroupIdAttribute == "" || strings.TrimSpace(groupUID) == "" {
		return nil, model.NewAppError("Ldap.GetGroup", "ent.ldap_groups.invalid_ldap_id", nil, "", http.StatusBadRequest)
	}

	ss, appErr := l.open(s)
	if appErr != nil {
		return nil, appErr
	}
	defer ss.Close()

	groups, err := fetchGroups(ss, andFilters(s.groupFilter(), equalityFilter(s.GroupIdAttribute, groupUID)), false)
	if err != nil {
		return nil, model.NewAppError("Ldap.GetGroup", "ent.ldap_groups.group_search_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if len(groups) == 0 {
		return nil, model.NewAppError("Ldap.GetGroup", "ent.ldap_groups.no_rows", nil, "", http.StatusNotFound)
	}
	return groups[0].toModel(), nil
}

// GetAllGroupsPage returns a page of the directory groups selected by the
// group filter, merged with the Mattermost groups they are linked to, sorted
// by display name.
func (l *Ldap) GetAllGroupsPage(rctx request.CTX, page int, perPage int, opts model.LdapGroupSearchOpts) ([]*model.Group, int, *model.AppError) {
	s := l.settings()
	if s.GroupIdAttribute == "" {
		return []*model.Group{}, 0, nil
	}

	ss, appErr := l.open(s)
	if appErr != nil {
		return nil, 0, appErr
	}
	defer ss.Close()

	entries, err := fetchGroups(ss, s.groupFilter(), false)
	if err != nil {
		return nil, 0, model.NewAppError("Ldap.GetAllGroupsPage", "ent.ldap_groups.groups_search_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	linked, err := l.b.GetLdapGroups()
	if err != nil {
		return nil, 0, model.NewAppError("Ldap.GetAllGroupsPage", "ent.ldap.syncronize.get_all_groups.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	linkedByRemoteID := make(map[string]*model.Group, len(linked))
	for _, g := range linked {
		if g.RemoteId != nil && g.DeleteAt == 0 {
			linkedByRemoteID[*g.RemoteId] = g
		}
	}

	q := strings.ToLower(strings.TrimSpace(opts.Q))
	result := make([]*model.Group, 0, len(entries))
	seen := make(map[string]bool, len(entries))
	for _, e := range entries {
		if seen[e.RemoteID] {
			continue
		}
		seen[e.RemoteID] = true

		if q != "" && !strings.Contains(strings.ToLower(e.DisplayName), q) {
			continue
		}

		g := e.toModel()
		if mm, ok := linkedByRemoteID[e.RemoteID]; ok {
			g.Id = mm.Id
			g.Name = mm.Name
			g.CreateAt = mm.CreateAt
			g.UpdateAt = mm.UpdateAt
			g.AllowReference = mm.AllowReference
			hasSyncables, err := l.b.GroupHasSyncables(mm.Id)
			if err != nil {
				return nil, 0, model.NewAppError("Ldap.GetAllGroupsPage", "ent.ldap.syncronize.get_all_groups.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
			}
			g.HasSyncables = hasSyncables
		}

		isLinked := g.Id != ""
		if opts.IsLinked != nil && *opts.IsLinked != isLinked {
			continue
		}
		if opts.IsConfigured != nil && *opts.IsConfigured != g.HasSyncables {
			continue
		}
		result = append(result, g)
	}

	sort.SliceStable(result, func(i, j int) bool {
		return strings.ToLower(result[i].DisplayName) < strings.ToLower(result[j].DisplayName)
	})

	total := len(result)
	if perPage <= 0 {
		return result, total, nil
	}
	start := page * perPage
	if start < 0 || start >= total {
		return []*model.Group{}, total, nil
	}
	end := min(start+perPage, total)
	return result[start:end], total, nil
}

// groupResolver computes the transitive user members of groups.
type groupResolver struct {
	groups map[string]*groupEntry // by normalized DN
	users  map[string]string      // normalized user DN -> Mattermost user ID
	memo   map[string]map[string]bool
}

func newGroupResolver(groups []*groupEntry, users map[string]string) *groupResolver {
	r := &groupResolver{
		groups: make(map[string]*groupEntry, len(groups)),
		users:  users,
		memo:   map[string]map[string]bool{},
	}
	for _, g := range groups {
		r.groups[g.DN] = g
	}
	return r
}

// members returns the Mattermost user IDs of the transitive members of the
// group with the given normalized DN.
func (r *groupResolver) members(dn string) map[string]bool {
	return r.resolve(dn, map[string]bool{}, 0)
}

func (r *groupResolver) resolve(dn string, visiting map[string]bool, depth int) map[string]bool {
	if m, ok := r.memo[dn]; ok {
		return m
	}
	res := map[string]bool{}
	g, ok := r.groups[dn]
	if !ok || visiting[dn] || depth > maxGroupNesting {
		return res
	}
	visiting[dn] = true
	defer delete(visiting, dn)

	for _, member := range g.Members {
		if userID, ok := r.users[member]; ok {
			res[userID] = true
			continue
		}
		if _, ok := r.groups[member]; ok {
			for userID := range r.resolve(member, visiting, depth+1) {
				res[userID] = true
			}
		}
	}
	// Only memoize complete results: results computed while inside a cycle
	// may be partial.
	if depth == 0 {
		r.memo[dn] = res
	}
	return res
}

// userGroupRemoteIDs returns the IDs of all the groups the user with the given
// DN belongs to, directly or through nested groups.
func userGroupRemoteIDs(ss *session, userDN string) (map[string]bool, error) {
	s := ss.s
	result := map[string]bool{}
	seenDNs := map[string]bool{normalizeDN(userDN): true}
	frontier := []string{userDN}

	for depth := 0; depth <= maxGroupNesting && len(frontier) > 0; depth++ {
		var next []string
		for start := 0; start < len(frontier); start += 50 {
			end := min(start+50, len(frontier))
			var sb strings.Builder
			sb.WriteString("(|")
			for _, dn := range frontier[start:end] {
				escaped := ldap.EscapeFilter(dn)
				for _, attr := range groupMemberAttributes {
					sb.WriteString("(" + attr + "=" + escaped + ")")
				}
			}
			sb.WriteString(")")

			entries, err := ss.search(andFilters(s.anyGroupFilter(), sb.String()), s.groupAttributes(false))
			if err != nil {
				return nil, err
			}
			for _, e := range entries {
				norm := normalizeDN(e.DN)
				if seenDNs[norm] {
					continue
				}
				seenDNs[norm] = true
				if id := attributeValue(e, s.GroupIdAttribute); id != "" {
					result[id] = true
				}
				next = append(next, e.DN)
			}
		}
		frontier = next
	}
	return result, nil
}

// syncUserGroups reconciles the memberships of a single user in the
// Mattermost groups linked to directory groups.
func (l *Ldap) syncUserGroups(rctx request.CTX, ss *session, userID, userDN string) *model.AppError {
	remoteIDs, err := userGroupRemoteIDs(ss, userDN)
	if err != nil {
		return model.NewAppError("Ldap.syncUserGroups", "ent.ldap_groups.reachable_groups_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	linked, err := l.b.GetLdapGroups()
	if err != nil {
		return model.NewAppError("Ldap.syncUserGroups", "ent.ldap.syncronize.get_all_groups.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	wanted := map[string]bool{}
	for _, g := range linked {
		if g.RemoteId != nil && remoteIDs[*g.RemoteId] {
			wanted[g.Id] = true
		}
	}

	current, err := l.b.GetLdapGroupsForUser(userID)
	if err != nil {
		return model.NewAppError("Ldap.syncUserGroups", "ent.ldap_groups.members_of_group_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	have := map[string]bool{}
	for _, g := range current {
		have[g.Id] = true
		if !wanted[g.Id] {
			if err := l.b.RemoveGroupMembers(g.Id, []string{userID}); err != nil {
				rctx.Logger().Warn("Failed to remove user from AD/LDAP group", mlog.String("group_id", g.Id), mlog.String("user_id", userID), mlog.Err(err))
			}
		}
	}
	for groupID := range wanted {
		if have[groupID] {
			continue
		}
		if err := l.b.AddGroupMembers(groupID, []string{userID}); err != nil {
			rctx.Logger().Warn("Failed to add user to AD/LDAP group", mlog.String("group_id", groupID), mlog.String("user_id", userID), mlog.Err(err))
		}
	}
	return nil
}
