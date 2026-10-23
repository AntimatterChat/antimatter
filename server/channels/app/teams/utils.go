// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package teams

import (
	"slices"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

// DefaultChannelNames gives the channels a new team is created with. By default the list will be (not necessarily in
// this order):
//
//	['general', 'welcome']
//
// However, if TeamSettings.ExperimentalDefaultChannels contains a list of channels then that list will be included in
// the return results in addition to 'general' and 'welcome'. For example:
//
//	['general', 'welcome', 'game-of-thrones', 'wow']
//
// Antimatter: General is where people land, Welcome gets the community's join and leave messages (Team.WelcomeChannelId);
// communities got Off-Topic instead of Welcome before. 'town-square', the default channel's old name, in that list
// stands for 'general' (no second channel).
func (ts *TeamService) DefaultChannelNames() []string {
	names := []string{model.DefaultChannelName, model.WelcomeChannelName}

	seenChannels := map[string]bool{model.DefaultChannelName: true, model.LegacyDefaultChannelName: true, model.WelcomeChannelName: true}
	for _, channelName := range ts.config().TeamSettings.ExperimentalDefaultChannels {
		if !seenChannels[channelName] {
			names = append(names, channelName)
			seenChannels[channelName] = true
		}
	}

	return names
}

func IsEmailAddressAllowed(email string, allowedDomains []string) bool {
	for _, restriction := range allowedDomains {
		domains := normalizeDomains(restriction)
		if len(domains) <= 0 {
			continue
		}
		matched := false
		for _, d := range domains {
			if strings.HasSuffix(email, "@"+d) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	return true
}

func (ts *TeamService) IsTeamEmailAllowed(user *model.User, team *model.Team) bool {
	if user.IsBot {
		return true
	}
	email := strings.ToLower(user.Email)
	allowedDomains := ts.GetAllowedDomains(user, team)
	return IsEmailAddressAllowed(email, allowedDomains)
}

func (ts *TeamService) GetAllowedDomains(user *model.User, team *model.Team) []string {
	if user.IsGuest() {
		return []string{*ts.config().GuestAccountsSettings.RestrictCreationToDomains}
	}
	// First check per team allowedDomains, then app wide restrictions
	return []string{team.AllowedDomains, *ts.config().TeamSettings.RestrictCreationToDomains}
}

func (ts *TeamService) checkValidDomains(team *model.Team) error {
	validDomains := normalizeDomains(*ts.config().TeamSettings.RestrictCreationToDomains)
	if len(validDomains) > 0 {
		for _, domain := range normalizeDomains(team.AllowedDomains) {
			matched := slices.Contains(validDomains, domain)
			if !matched {
				return &DomainError{Domain: domain}
			}
		}
	}

	return nil
}

func normalizeDomains(domains string) []string {
	// commas and @ signs are optional
	// can be in the form of "@corp.mattermost.com, mattermost.com mattermost.org" -> corp.mattermost.com mattermost.com mattermost.org
	return strings.Fields(strings.TrimSpace(strings.ToLower(strings.Replace(strings.Replace(domains, "@", " ", -1), ",", " ", -1))))
}

// UserIsInAdminRoleGroup returns true at least one of the user's groups are configured to set the members as
// admins in the given syncable.
func (ts *TeamService) userIsInAdminRoleGroup(userID, syncableID string, syncableType model.GroupSyncableType) (bool, error) {
	groupIDs, err := ts.groupStore.AdminRoleGroupsForSyncableMember(userID, syncableID, syncableType)
	if err != nil {
		return false, err
	}

	if len(groupIDs) == 0 {
		return false, nil
	}

	return true, nil
}
