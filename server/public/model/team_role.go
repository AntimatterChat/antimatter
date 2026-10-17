// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package model

import (
	"slices"
	"strings"
)

// An Antimatter role (a group of one team) grants its members a Mattermost role in that team, named after the group,
// holding the permissions its editor's switches give. TeamRolePermissions are the permissions it may hold: team and
// channel ones a team's admins can hand out, nothing system-wide. (A function: the permissions are set up at init.)
func TeamRolePermissions() []string {
	return []string{
		PermissionManageTeam.Id,
		PermissionManageTeamRoles.Id,
		PermissionAddUserToTeam.Id,
		PermissionRemoveUserFromTeam.Id,
		PermissionCreatePublicChannel.Id,
		PermissionCreatePrivateChannel.Id,
		PermissionManagePublicChannelProperties.Id,
		PermissionManagePrivateChannelProperties.Id,
		PermissionDeletePublicChannel.Id,
		PermissionDeletePrivateChannel.Id,
		PermissionManagePublicChannelMembers.Id,
		PermissionManagePrivateChannelMembers.Id,
		PermissionManageChannelRoles.Id,
		PermissionConvertPublicChannelToPrivate.Id,
		PermissionConvertPrivateChannelToPublic.Id,
		// Antimatter: the community-level permissions of the channels' admins, so that a role (Administrator
		// first) holds what the community's admins do. The webhook channel-lock bypass and the attribute-based
		// access rules stay with the admins' roles.
		PermissionManagePublicChannelBanner.Id,
		PermissionManagePrivateChannelBanner.Id,
		PermissionManageChannelJoinRequests.Id,
		PermissionManagePrivateChannelDiscoverability.Id,
		PermissionManagePublicChannelAutoTranslation.Id,
		PermissionManagePrivateChannelAutoTranslation.Id,
		PermissionDeleteOthersPosts.Id,
		PermissionEditOthersPosts.Id,
		PermissionManageOwnIncomingWebhooks.Id,
		PermissionManageOwnOutgoingWebhooks.Id,
		PermissionManageOwnSlashCommands.Id,
		PermissionManageOthersIncomingWebhooks.Id,
		PermissionManageOthersOutgoingWebhooks.Id,
		PermissionManageOthersSlashCommands.Id,
	}
}

const teamRolePrefix = "team_role_"

// TeamRoleName is the name of the Mattermost role an Antimatter role grants.
func TeamRoleName(groupID string) string {
	return teamRolePrefix + groupID
}

// IsTeamRoleName tells whether a Mattermost role is one an Antimatter role grants.
func IsTeamRoleName(name string) bool {
	return strings.HasPrefix(name, teamRolePrefix)
}

// AreTeamRolePermissions tells whether every permission is one a role may grant.
func AreTeamRolePermissions(permissions []string) bool {
	for _, p := range permissions {
		if !slices.Contains(TeamRolePermissions(), p) {
			return false
		}
	}
	return true
}
