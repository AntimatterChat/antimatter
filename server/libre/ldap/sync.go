// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package ldap

import (
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/mlog"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// Keys of the job data, as displayed by the System Console.
const (
	jobDataUsersCount             = "ldap_users_count"
	jobDataUpdateCount            = "update_count"
	jobDataDeleteCount            = "delete_count"
	jobDataTotalGroupsCount       = "total_ldap_groups_count"
	jobDataGroupDeleteCount       = "group_delete_count"
	jobDataGroupMemberDeleteCount = "group_member_delete_count"
	jobDataGroupMemberAddCount    = "group_member_add_count"

	// jobDataIncludeRemovedMembers optionally overrides
	// LdapSettings.ReAddRemovedMembers for a single job.
	jobDataIncludeRemovedMembers = "include_removed_members"
)

// syncStats holds the counters of a synchronization.
type syncStats struct {
	LdapUsers          int
	Updated            int
	Deactivated        int
	TotalGroups        int
	GroupsDeleted      int
	GroupMembersAdded  int
	GroupMembersRemove int
}

func (st *syncStats) toJobData(data model.StringMap) model.StringMap {
	if data == nil {
		data = model.StringMap{}
	}
	data[jobDataUsersCount] = strconv.Itoa(st.LdapUsers)
	data[jobDataUpdateCount] = strconv.Itoa(st.Updated)
	data[jobDataDeleteCount] = strconv.Itoa(st.Deactivated)
	data[jobDataTotalGroupsCount] = strconv.Itoa(st.TotalGroups)
	data[jobDataGroupDeleteCount] = strconv.Itoa(st.GroupsDeleted)
	data[jobDataGroupMemberDeleteCount] = strconv.Itoa(st.GroupMembersRemove)
	data[jobDataGroupMemberAddCount] = strconv.Itoa(st.GroupMembersAdded)
	return data
}

// syncOptions tunes a synchronization.
type syncOptions struct {
	ReAddRemovedMembers bool
}

// Synchronize runs a full synchronization of users and groups with the
// directory.
func (l *Ldap) Synchronize(rctx request.CTX, opts syncOptions) (*syncStats, *model.AppError) {
	l.syncMutex.Lock()
	defer l.syncMutex.Unlock()

	s := l.settings()
	cfg := l.b.Config()
	logger := rctx.Logger()
	stats := &syncStats{}

	ss, appErr := l.open(s)
	if appErr != nil {
		return stats, appErr
	}
	defer ss.Close()

	// Start time of the previous successful synchronization, used to add the
	// new group members to teams and channels.
	var since int64
	if lastJob, err := l.b.GetLastSuccessfulSyncJob(); err != nil {
		logger.Warn("Unable to find the previous AD/LDAP synchronization", mlog.Err(err))
	} else if lastJob != nil {
		since = lastJob.StartAt
	}

	cpa, err := l.loadCPAMapping(rctx)
	if err != nil {
		logger.Warn("Unable to load user attribute fields synchronized with AD/LDAP", mlog.Err(err))
		cpa = nil
	}

	// 1. Read the users selected by the user filter.
	entries, err := ss.search(s.userFilter(), s.userAttributes(cpa.attributes()...))
	if err != nil {
		return stats, searchError("Ldap.Synchronize", err)
	}
	byID := make(map[string]*ldapUser, len(entries))
	byEmail := make(map[string]*ldapUser, len(entries))
	for _, e := range entries {
		lu := newLdapUser(s, e)
		if lu.ID == "" {
			continue
		}
		byID[lu.ID] = lu
		if s.EmailAttribute != "" {
			if email := strings.ToLower(strings.TrimSpace(attributeValue(e, s.EmailAttribute))); email != "" {
				byEmail[email] = lu
			}
		}
	}
	stats.LdapUsers = len(byID)

	// Refuse to deactivate every account when the directory unexpectedly
	// returns no users, which usually denotes a configuration problem.
	ldapUsers, err := l.b.GetUsersByAuthService(model.UserAuthServiceLdap)
	if err != nil {
		return stats, model.NewAppError("Ldap.Synchronize", "ent.ldap.syncronize.get_all.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	if len(byID) == 0 && slices.ContainsFunc(ldapUsers, func(u *model.User) bool { return u.DeleteAt == 0 }) {
		return stats, model.NewAppError("Ldap.Synchronize", "ent.ldap.no.users.checkcertificate", nil, "", http.StatusInternalServerError)
	}

	guestDNs, appErr := l.guestDNs(ss, cfg)
	if appErr != nil {
		return stats, appErr
	}

	// Normalized user DN -> Mattermost user ID, used to resolve groups.
	userIDsByDN := map[string]string{}

	// 2. Update the AD/LDAP users.
	for _, user := range ldapUsers {
		var lu *ldapUser
		if user.AuthData != nil {
			lu = byID[*user.AuthData]
		}
		l.syncUser(rctx, s, user, lu, true, guestDNs, cpa, stats)
		if lu != nil {
			userIDsByDN[normalizeDN(lu.DN)] = user.Id
		}
	}

	// 3. Update the SAML users synchronized with AD/LDAP.
	if model.SafeDereference(cfg.SamlSettings.EnableSyncWithLdap) {
		samlUsers, err := l.b.GetUsersByAuthService(model.UserAuthServiceSaml)
		if err != nil {
			return stats, model.NewAppError("Ldap.Synchronize", "ent.ldap.syncronize.get_all.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		ignoreGuests := model.SafeDereference(cfg.SamlSettings.IgnoreGuestsLdapSync)
		bindWithID := samlBindsWithID(cfg)
		includeAuth := model.SafeDereference(cfg.SamlSettings.EnableSyncWithLdapIncludeAuth)
		for _, user := range samlUsers {
			if ignoreGuests && user.IsGuest() {
				continue
			}
			var lu *ldapUser
			if bindWithID && user.AuthData != nil {
				lu = byID[*user.AuthData]
			}
			if lu == nil {
				lu = byEmail[strings.ToLower(user.Email)]
			}
			l.syncSamlUser(rctx, s, user, lu, includeAuth, bindWithID, cpa, stats)
			if lu != nil {
				userIDsByDN[normalizeDN(lu.DN)] = user.Id
			}
		}
	}

	// 4. Synchronize the groups.
	if s.GroupIdAttribute != "" {
		groupIDs, appErr := l.syncGroups(rctx, ss, userIDsByDN, stats)
		if appErr != nil {
			return stats, appErr
		}

		// 5. Apply the group memberships to teams and channels.
		params := model.CreateDefaultMembershipParams{
			Since:               since,
			ReAddRemovedMembers: opts.ReAddRemovedMembers,
		}
		if err := l.b.CreateDefaultMemberships(rctx, params); err != nil {
			logger.Warn("Failed to create default team and channel memberships", mlog.Err(err))
		}
		if err := l.b.DeleteGroupConstrainedMemberships(rctx); err != nil {
			logger.Warn("Failed to remove group constrained team and channel memberships", mlog.Err(err))
		}
		if err := l.b.SyncGroupConstrainedRoles(rctx, groupIDs); err != nil {
			logger.Warn("Failed to synchronize team and channel roles of groups", mlog.Err(err))
		}
	}

	logger.LogM(mlog.MlvlLDAPInfo, "AD/LDAP synchronization completed",
		mlog.Int("ldap_users", stats.LdapUsers),
		mlog.Int("updated_users", stats.Updated),
		mlog.Int("deactivated_users", stats.Deactivated),
		mlog.Int("groups", stats.TotalGroups),
		mlog.Int("deleted_groups", stats.GroupsDeleted),
		mlog.Int("added_group_members", stats.GroupMembersAdded),
		mlog.Int("removed_group_members", stats.GroupMembersRemove),
	)
	return stats, nil
}

// guestDNs returns the set of normalized DNs selected by the guest filter, or
// nil when the guest filter is not in use.
func (l *Ldap) guestDNs(ss *session, cfg *model.Config) (map[string]bool, *model.AppError) {
	s := ss.s
	if !guestFilterEnabled(cfg, s) {
		return nil, nil
	}
	entries, err := ss.search(andFilters(s.userFilter(), s.GuestFilter), []string{"1.1"})
	if err != nil {
		return nil, searchError("Ldap.guestDNs", err)
	}
	res := make(map[string]bool, len(entries))
	for _, e := range entries {
		res[normalizeDN(e.DN)] = true
	}
	return res, nil
}

// syncUser updates, reactivates or deactivates an AD/LDAP user. lu is nil
// when the user is not selected by the user filter anymore.
func (l *Ldap) syncUser(rctx request.CTX, s *settings, user *model.User, lu *ldapUser, includeIdentity bool, guestDNs map[string]bool, cpa *cpaMapping, stats *syncStats) {
	logger := rctx.Logger().With(mlog.String("user_id", user.Id))

	if lu == nil {
		if user.DeleteAt == 0 {
			if err := l.b.SetUserActive(rctx, user, false); err != nil {
				logger.Warn("Failed to deactivate user removed from AD/LDAP", mlog.Err(err))
				return
			}
			logger.LogM(mlog.MlvlLDAPInfo, "Deactivated user not found in AD/LDAP")
			stats.Deactivated++
		}
		return
	}

	updated := user.DeepCopy()
	changed := applyProfile(updated, profileFromEntry(s, lu.Entry), includeIdentity)
	if changed {
		saved, err := l.b.SaveUser(rctx, updated)
		if err != nil {
			logger.Warn("Failed to update user from AD/LDAP", mlog.Err(err))
		} else {
			user = saved
		}
	}

	reactivated := false
	if user.DeleteAt != 0 {
		if err := l.b.SetUserActive(rctx, user, true); err != nil {
			logger.Warn("Failed to reactivate user found in AD/LDAP", mlog.Err(err))
		} else {
			reactivated = true
			logger.LogM(mlog.MlvlLDAPInfo, "Reactivated user found in AD/LDAP")
		}
	}

	if guestDNs != nil && guestDNs[normalizeDN(lu.DN)] && !user.IsGuest() && !user.IsSystemAdmin() && user.DeleteAt == 0 {
		if err := l.b.DemoteUserToGuest(rctx, user); err != nil {
			logger.Warn("Failed to demote user matching the AD/LDAP guest filter", mlog.Err(err))
		} else {
			changed = true
		}
	}

	if changed || reactivated {
		stats.Updated++
	}

	if cpa != nil {
		if err := l.syncCPAWithMapping(rctx, cpa, user.Id, lu.Entry); err != nil {
			logger.Warn("Failed to synchronize user attributes from AD/LDAP", mlog.Err(err))
		}
	}
}

// syncSamlUser updates or deactivates a SAML user synchronized with AD/LDAP.
func (l *Ldap) syncSamlUser(rctx request.CTX, s *settings, user *model.User, lu *ldapUser, includeAuth, bindWithID bool, cpa *cpaMapping, stats *syncStats) {
	if lu == nil {
		l.syncUser(rctx, s, user, nil, false, nil, nil, stats)
		return
	}

	// When overriding SAML bind data, the SAML account is bound to the AD/LDAP
	// ID attribute if the SAML ID attribute is configured, or to the AD/LDAP
	// email otherwise.
	if includeAuth && user.DeleteAt == 0 {
		if bindWithID {
			if user.AuthData == nil || *user.AuthData != lu.ID {
				if err := l.b.UpdateAuthData(user.Id, model.UserAuthServiceSaml, lu.ID); err != nil {
					rctx.Logger().Warn("Failed to bind SAML user to AD/LDAP ID", mlog.String("user_id", user.Id), mlog.Err(err))
				} else {
					user.AuthData = &lu.ID
				}
			}
		} else if p := profileFromEntry(s, lu.Entry); p.Email != nil && *p.Email != user.Email {
			updated := user.DeepCopy()
			updated.Email = *p.Email
			if saved, err := l.b.SaveUser(rctx, updated); err != nil {
				rctx.Logger().Warn("Failed to override SAML email with AD/LDAP email", mlog.String("user_id", user.Id), mlog.Err(err))
			} else {
				user = saved
				stats.Updated++
			}
		}
	}

	l.syncUser(rctx, s, user, lu, false, nil, cpa, stats)
}

// syncGroups synchronizes the Mattermost groups linked to directory groups.
// It returns the IDs of the Mattermost groups still linked.
func (l *Ldap) syncGroups(rctx request.CTX, ss *session, userIDsByDN map[string]string, stats *syncStats) ([]string, *model.AppError) {
	s := ss.s
	logger := rctx.Logger()

	available, err := fetchGroups(ss, s.groupFilter(), true)
	if err != nil {
		return nil, model.NewAppError("Ldap.syncGroups", "ent.ldap.syncronize.get_all_groups.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}
	stats.TotalGroups = len(available)

	all := available
	if s.GroupFilter != "" {
		// Nested groups may not be selected by the configured group filter.
		nested, err := fetchGroups(ss, s.anyGroupFilter(), true)
		if err != nil {
			return nil, model.NewAppError("Ldap.syncGroups", "ent.ldap.syncronize.get_all_groups.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
		}
		all = append(slices.Clone(available), nested...)
	}
	resolver := newGroupResolver(all, userIDsByDN)

	byRemoteID := make(map[string]*groupEntry, len(available))
	for _, g := range available {
		if _, ok := byRemoteID[g.RemoteID]; !ok {
			byRemoteID[g.RemoteID] = g
		}
	}

	linked, err := l.b.GetLdapGroups()
	if err != nil {
		return nil, model.NewAppError("Ldap.syncGroups", "ent.ldap.syncronize.get_all_groups.app_error", nil, "", http.StatusInternalServerError).Wrap(err)
	}

	var kept []string
	for _, group := range linked {
		if group.DeleteAt != 0 || group.RemoteId == nil {
			continue
		}
		glog := logger.With(mlog.String("group_id", group.Id))

		entry, ok := byRemoteID[*group.RemoteId]
		if !ok {
			if err := l.b.DeleteGroup(group.Id); err != nil {
				glog.Warn("Failed to delete group removed from AD/LDAP", mlog.Err(err))
				continue
			}
			glog.LogM(mlog.MlvlLDAPInfo, "Deleted group not found in AD/LDAP anymore")
			stats.GroupsDeleted++
			continue
		}
		kept = append(kept, group.Id)

		if name := entry.toModel().DisplayName; name != "" && name != group.DisplayName {
			group.DisplayName = name
			if err := l.b.UpdateGroup(group); err != nil {
				glog.Warn("Failed to update group display name", mlog.Err(err))
			}
		}

		wanted := resolver.members(entry.DN)
		current, err := l.b.GetGroupMemberIDs(group.Id)
		if err != nil {
			glog.Warn("Failed to get group members", mlog.Err(err))
			continue
		}
		have := make(map[string]bool, len(current))
		var toRemove []string
		for _, id := range current {
			have[id] = true
			if !wanted[id] {
				toRemove = append(toRemove, id)
			}
		}
		var toAdd []string
		for id := range wanted {
			if !have[id] {
				toAdd = append(toAdd, id)
			}
		}
		slices.Sort(toAdd)

		if len(toRemove) > 0 {
			if err := l.b.RemoveGroupMembers(group.Id, toRemove); err != nil {
				glog.Warn("Failed to remove group members", mlog.Err(err))
			} else {
				stats.GroupMembersRemove += len(toRemove)
			}
		}
		if len(toAdd) > 0 {
			if err := l.b.AddGroupMembers(group.Id, toAdd); err != nil {
				glog.Warn("Failed to add group members", mlog.Err(err))
			} else {
				stats.GroupMembersAdded += len(toAdd)
			}
		}
	}
	return kept, nil
}
