// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Channel, ChannelTypeEntry} from '@mattermost/types/channels';
import type {ChannelOverride} from '@mattermost/types/channel_overrides';
import type {GlobalState} from '@mattermost/types/store';

import {Permissions} from 'mattermost-redux/constants';

import {canStartConversations, isChannelModerator} from './roles';

// Parity with the server's rule 2 (channel_posting_rules.go, App.CanStartConversations): the client reads the same
// channel type settings and overrides, so the composer can hide itself without a round trip. Cases mirror the
// server's RBAC regression (channel_posting_rules_test.go: TestRootPostsInAnnouncementChannels,
// TestRootPostsInChatChannels, TestRootPostsInAnnouncementForums).

const dm = (): Channel => ({id: 'dm', type: 'D'} as Channel);
const channel = (id = 'dev'): Channel => ({id, type: 'O', team_id: 'team'} as Channel);

const override = (targetType: 'role' | 'user', targetId: string, allow: string[], deny: string[]): ChannelOverride =>
    ({channel_id: 'dev', target_type: targetType, target_id: targetId, allow, deny, update_at: 0});

type Options = {
    rootPosts?: 'members' | 'moderators';
    kind?: 'text' | 'voice' | 'forum'; // the Antimatter channel type; the rule is the same for every one
    teamRoles?: string; // space-separated role names on the team membership
    systemRoles?: string; // space-separated role names on the user
    overrides?: ChannelOverride[];
    timedOut?: boolean;
    canPost?: boolean; // whether the base channel_user role has create_post
};

function stateWith({rootPosts = 'members', kind = 'text', teamRoles = 'team_user', systemRoles = 'system_user', overrides = [], timedOut = false, canPost = true}: Options): GlobalState {
    const channelTypes: Record<string, ChannelTypeEntry> = {dev: {type: kind, mode: 'chat', root_posts: rootPosts, preset: ''}};
    return {
        entities: {
            general: {serverVersion: ''},
            users: {currentUserId: 'me', profiles: {me: {id: 'me', roles: systemRoles}}},
            teams: {
                currentTeamId: 'team',
                myMembers: {team: {team_id: 'team', user_id: 'me', roles: teamRoles}},
                timeouts: timedOut ? {team: [{team_id: 'team', user_id: 'me', until: Date.now() + 100000, reason: '', created_by: 'mod', create_at: 0}]} : {},
            },
            channels: {
                currentChannelId: 'dev',
                channelTypes,
                myMembers: {dev: {channel_id: 'dev', user_id: 'me', roles: 'channel_user'}},
                roles: {dev: new Set(['channel_user'])},
                overrides: {dev: overrides},
            },
            roles: {
                roles: {
                    system_user: {name: 'system_user', permissions: []},
                    system_admin: {name: 'system_admin', permissions: [Permissions.MANAGE_SYSTEM]},
                    team_user: {name: 'team_user', permissions: []},
                    team_admin: {name: 'team_admin', permissions: [Permissions.MANAGE_TEAM]},
                    team_role_mods: {name: 'team_role_mods', permissions: [Permissions.DELETE_OTHERS_POSTS]},
                    team_role_writer: {name: 'team_role_writer', permissions: []},
                    channel_user: {name: 'channel_user', permissions: canPost ? [Permissions.CREATE_POST] : []},
                },
            },
        },
    } as unknown as GlobalState;
}

describe('canStartConversations (Antimatter channel types, §2.1)', () => {
    test('direct and group messages have no such rule', () => {
        expect(canStartConversations(stateWith({}), dm())).toBe(true);
    });

    test('members start conversations where root_posts is members, and no override is about it', () => {
        expect(canStartConversations(stateWith({rootPosts: 'members'}), channel())).toBe(true);
    });

    test('a member can\'t start a conversation in an announcement channel, but can reply (create_post stands)', () => {
        expect(canStartConversations(stateWith({rootPosts: 'moderators'}), channel())).toBe(false);
    });

    test('the people who moderate the channel start conversations: a team admin', () => {
        expect(canStartConversations(stateWith({rootPosts: 'moderators', teamRoles: 'team_user team_admin'}), channel())).toBe(true);
    });

    test('the people who moderate the channel start conversations: a system admin', () => {
        expect(canStartConversations(stateWith({rootPosts: 'moderators', systemRoles: 'system_user system_admin'}), channel())).toBe(true);
    });

    test('the people who moderate the channel start conversations: a role that manages messages', () => {
        expect(canStartConversations(stateWith({rootPosts: 'moderators', teamRoles: 'team_user team_role_mods'}), channel())).toBe(true);
    });

    test('an override allowing create_root_post to a role succeeds', () => {
        const overrides = [override('role', 'writer', [Permissions.CREATE_ROOT_POST], [])];
        expect(canStartConversations(stateWith({rootPosts: 'moderators', teamRoles: 'team_user team_role_writer', overrides}), channel())).toBe(true);
    });

    test('a deny to the person beats an allow to their role', () => {
        const overrides = [
            override('role', 'writer', [Permissions.CREATE_ROOT_POST], []),
            override('user', 'me', [], [Permissions.CREATE_ROOT_POST]),
        ];
        expect(canStartConversations(stateWith({rootPosts: 'moderators', teamRoles: 'team_user team_role_writer', overrides}), channel())).toBe(false);
    });

    test('an override denying create_root_post in a chat channel leaves only replies', () => {
        const overrides = [override('user', 'me', [], [Permissions.CREATE_ROOT_POST])];
        expect(canStartConversations(stateWith({rootPosts: 'members', overrides}), channel())).toBe(false);
    });

    test('team admins pass over a role\'s deny, as elsewhere', () => {
        const overrides = [override('role', 'writer', [], [Permissions.CREATE_ROOT_POST])];
        expect(canStartConversations(stateWith({rootPosts: 'members', teamRoles: 'team_user team_role_writer team_admin', overrides}), channel())).toBe(true);
    });

    test('a timed-out moderator still can\'t: create_post itself is timed out', () => {
        expect(canStartConversations(stateWith({rootPosts: 'moderators', teamRoles: 'team_user team_role_mods', timedOut: true}), channel())).toBe(false);
    });

    test('members with no override at all answer at once, whoever may post there (the general create_post check elsewhere still applies)', () => {
        expect(canStartConversations(stateWith({rootPosts: 'members', canPost: false}), channel())).toBe(true);
    });

    test('once an override is in play, create_post is still needed, even where root_posts is members', () => {
        const overrides = [override('role', 'someone-else', [Permissions.CREATE_ROOT_POST], [])];
        expect(canStartConversations(stateWith({rootPosts: 'members', canPost: false, overrides}), channel())).toBe(false);
    });

    test('a forum\'s Announcements preset is the same rule: moderators open topics', () => {
        expect(canStartConversations(stateWith({rootPosts: 'moderators', kind: 'forum'}), channel())).toBe(false);
        expect(canStartConversations(stateWith({rootPosts: 'moderators', kind: 'forum', teamRoles: 'team_user team_admin'}), channel())).toBe(true);
    });
});

describe('isChannelModerator (Antimatter channel types, §2.1)', () => {
    test('a plain member isn\'t one', () => {
        expect(isChannelModerator(stateWith({}), 'team', 'dev')).toBe(false);
    });

    test('system admins, community admins and roles that manage messages are', () => {
        expect(isChannelModerator(stateWith({systemRoles: 'system_user system_admin'}), 'team', 'dev')).toBe(true);
        expect(isChannelModerator(stateWith({teamRoles: 'team_user team_admin'}), 'team', 'dev')).toBe(true);
        expect(isChannelModerator(stateWith({teamRoles: 'team_user team_role_mods'}), 'team', 'dev')).toBe(true);
    });
});
