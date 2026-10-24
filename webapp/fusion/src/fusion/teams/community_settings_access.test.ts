// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {Permissions} from 'mattermost-redux/constants';

import {canManageTeamIntegrations, canOpenCommunitySettings, getCommunitySettingsAccess} from './community_settings_access';
import {accessState} from './settings_test_utils';

// Who gets the community settings, and which of their pages: the community menu and the settings dialog both ask this.
describe('fusion/teams/community_settings_access', () => {
    const access = (perms: string[], options?: Parameters<typeof accessState>[1]) => getCommunitySettingsAccess(accessState(perms, options), 't1');

    test('a member without rights gets no settings', () => {
        // The default member: invites, adds people, creates channels.
        const member = access([Permissions.INVITE_USER, Permissions.ADD_USER_TO_TEAM, Permissions.CREATE_PUBLIC_CHANNEL]);
        expect(canOpenCommunitySettings(member)).toBe(false);
        expect(member).toMatchObject({manage: false, access: false, manageMembers: false, bans: false, roles: false, aiConnections: false, integrations: false});
    });

    test('a channel admin manages nothing of the community: no settings', () => {
        const channelAdmin = access([Permissions.INVITE_USER], {channelAdmin: true});
        expect(canOpenCommunitySettings(channelAdmin)).toBe(false);
        expect(channelAdmin.integrations).toBe(false);
    });

    test('the community\'s admins get every page', () => {
        const admin = access([Permissions.MANAGE_TEAM, Permissions.MANAGE_TEAM_ROLES, Permissions.INVITE_USER, Permissions.MANAGE_TEAM_AI_CONNECTIONS, Permissions.MANAGE_OWN_INCOMING_WEBHOOKS]);
        expect(canOpenCommunitySettings(admin)).toBe(true);
        expect(admin).toMatchObject({manage: true, access: true, manageMembers: true, teams: true, roles: true, aiConnections: true, integrations: true});
    });

    test('managing roles alone opens the settings, for the members\' roles and the community\'s roles', () => {
        const roles = access([Permissions.MANAGE_TEAM_ROLES]);
        expect(canOpenCommunitySettings(roles)).toBe(true);
        expect(roles).toMatchObject({manage: false, access: false, manageMembers: true, roles: true, integrations: false});

        // Without teams and roles on the server, still the members' roles.
        const noGroups = access([Permissions.MANAGE_TEAM_ROLES], {customGroups: false});
        expect(canOpenCommunitySettings(noGroups)).toBe(true);
        expect(noGroups).toMatchObject({manageMembers: true, roles: false, teams: false});
    });

    // RBAC: the bans are for those who may ban; a moderator of messages (the mod view, no removals) doesn't get them.
    test('removing members alone opens the settings, for the bans on the Members page', () => {
        const moderator = access([Permissions.REMOVE_USER_FROM_TEAM]);
        expect(canOpenCommunitySettings(moderator)).toBe(true);
        expect(moderator).toMatchObject({manage: false, access: false, manageMembers: false, roles: false, bans: true});
        expect(access([Permissions.MANAGE_TEAM]).bans).toBe(true);
        expect(access([Permissions.DELETE_OTHERS_POSTS]).bans).toBe(false);
        expect(canOpenCommunitySettings(access([Permissions.DELETE_OTHERS_POSTS]))).toBe(false);
        expect(access([Permissions.INVITE_USER, Permissions.ADD_USER_TO_TEAM]).bans).toBe(false);
    });

    test('model connections alone open the settings for them', () => {
        const ai = access([Permissions.MANAGE_TEAM_AI_CONNECTIONS]);
        expect(canOpenCommunitySettings(ai)).toBe(true);
        expect(ai).toMatchObject({manage: false, aiConnections: true, roles: false, manageMembers: false});
    });

    test('inviting isn\'t managing: the Access page takes managing the community too', () => {
        expect(access([Permissions.INVITE_USER]).access).toBe(false);
        expect(access([Permissions.MANAGE_TEAM]).access).toBe(false);
        expect(access([Permissions.MANAGE_TEAM, Permissions.INVITE_USER]).access).toBe(true);
    });

    describe('integrations, as their pages check them', () => {
        test.each([
            Permissions.MANAGE_SLASH_COMMANDS,
            Permissions.MANAGE_OWN_SLASH_COMMANDS,
            Permissions.MANAGE_INCOMING_WEBHOOKS,
            Permissions.MANAGE_OWN_INCOMING_WEBHOOKS,
            Permissions.MANAGE_OUTGOING_WEBHOOKS,
            Permissions.MANAGE_OWN_OUTGOING_WEBHOOKS,
        ])('%s in the community opens the settings for them', (permission) => {
            const state = accessState([permission]);
            expect(canManageTeamIntegrations(state, 't1')).toBe(true);
            expect(canOpenCommunitySettings(getCommunitySettingsAccess(state, 't1'))).toBe(true);
        });

        test.each([Permissions.MANAGE_OAUTH, Permissions.MANAGE_BOTS, Permissions.MANAGE_OTHERS_BOTS])('%s on the server too', (permission) => {
            expect(canManageTeamIntegrations(accessState([], {systemPermissions: [permission]}), 't1')).toBe(true);
        });

        test('custom emoji are not the community\'s integrations', () => {
            const state = accessState([], {systemPermissions: [Permissions.CREATE_EMOJIS, Permissions.DELETE_EMOJIS]});
            expect(canManageTeamIntegrations(state, 't1')).toBe(false);
            expect(canOpenCommunitySettings(getCommunitySettingsAccess(state, 't1'))).toBe(false);
        });
    });
});
