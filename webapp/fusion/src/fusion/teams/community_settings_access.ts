// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {Permissions} from 'mattermost-redux/constants';
import {isCustomGroupsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import {haveISystemPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';

import {isChannelAccessControlEnabled, isTeamMembershipAccessControlEnabled} from 'selectors/general';

import type {GlobalState} from 'types/store';

// What one may do in a community's settings, page by page: the same rights for its community menu (who sees
// Community settings) and for the settings dialog (which pages it shows).
export type CommunitySettingsAccess = {

    // The community's own settings: its name and picture, channels, onboarding, attributes, layout, autolinks.
    manage: boolean;

    // Its invitations (allowed domains, open invite, the invite link), for those who manage it and may invite.
    access: boolean;

    // Who may join, and the channels' membership policies, by user attributes (ABAC), when the server has it.
    teamMembership: boolean;
    accessPolicies: boolean;

    // Members: everyone may see them (the former View members); those who manage the community's roles also make
    // people its admins or members and remove them there (the former Manage members).
    manageMembers: boolean;

    // Teams (a community's user groups) when the server has them: everyone may see them and, as the server allows,
    // create their own; they aren't a reason on their own to show the settings.
    teams: boolean;

    // The community's roles, and its own model connections.
    roles: boolean;
    aiConnections: boolean;

    // Its integrations (webhooks, slash commands, OAuth apps, bot accounts): a page of their own the settings lead to.
    integrations: boolean;
};

// canManageTeamIntegrations: whether the integrations pages have integrations for you to manage in the community, as
// the integrations pages (and the classic backstage) check it. Custom emoji, and the apps' own uploads (custom GIFs,
// stickers), aren't the community's: the pickers manage them.
export function canManageTeamIntegrations(state: GlobalState, teamId: string): boolean {
    const inTeam = (permission: string) => Boolean(haveITeamPermission(state, teamId, permission));
    const onServer = (permission: string) => haveISystemPermission(state, {permission});
    return inTeam(Permissions.MANAGE_SLASH_COMMANDS) || inTeam(Permissions.MANAGE_OWN_SLASH_COMMANDS) ||
        inTeam(Permissions.MANAGE_INCOMING_WEBHOOKS) || inTeam(Permissions.MANAGE_OWN_INCOMING_WEBHOOKS) ||
        inTeam(Permissions.MANAGE_OUTGOING_WEBHOOKS) || inTeam(Permissions.MANAGE_OWN_OUTGOING_WEBHOOKS) ||
        onServer(Permissions.MANAGE_OAUTH) || onServer(Permissions.MANAGE_BOTS) || onServer(Permissions.MANAGE_OTHERS_BOTS);
}

export function getCommunitySettingsAccess(state: GlobalState, teamId: string): CommunitySettingsAccess {
    const inTeam = (permission: string) => Boolean(haveITeamPermission(state, teamId, permission));
    const manage = inTeam(Permissions.MANAGE_TEAM);
    const customGroups = isCustomGroupsEnabled(state);
    const accessRules = haveISystemPermission(state, {permission: Permissions.MANAGE_SYSTEM}) || inTeam(Permissions.MANAGE_TEAM_ACCESS_RULES);
    const manageRoles = inTeam(Permissions.MANAGE_TEAM_ROLES);
    return {
        manage,
        access: manage && inTeam(Permissions.INVITE_USER),
        teamMembership: Boolean(isTeamMembershipAccessControlEnabled(state)) && accessRules,
        accessPolicies: Boolean(isChannelAccessControlEnabled(state)) && accessRules,
        manageMembers: manageRoles,
        teams: customGroups,
        roles: manageRoles && customGroups,
        aiConnections: inTeam(Permissions.MANAGE_TEAM_AI_CONNECTIONS),
        integrations: canManageTeamIntegrations(state, teamId),
    };
}

// canOpenCommunitySettings: whether one may manage something in the community's settings, which its menu then
// offers. Seeing its members and teams isn't enough: those who only do that find them in the menu itself.
export function canOpenCommunitySettings(access: CommunitySettingsAccess): boolean {
    return access.manage || access.teamMembership || access.accessPolicies || access.manageMembers || access.roles ||
        access.aiConnections || access.integrations;
}
