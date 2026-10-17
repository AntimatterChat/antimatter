// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {GlobalState} from 'types/store';

import {canCreateCustomEmoji, canDeleteOthersCustomEmoji, canDeleteOwnCustomEmoji, canManageCustomEmoji} from './custom_emoji';

// The rules of api4/emoji.go: custom emoji on, and the permission from the system role or the role in any community.
// Deleting someone else's emoji takes delete_emojis and delete_others_emojis, wherever each comes from.

type Setup = {
    enabled?: boolean;
    systemRole?: string;
    system?: string[];
    team?: string[];
};

function state({enabled = true, systemRole = 'system_user', system = [], team = []}: Setup): GlobalState {
    return {
        entities: {
            general: {config: {EnableCustomEmoji: enabled ? 'true' : 'false'}},
            users: {currentUserId: 'me', profiles: {me: {id: 'me', roles: systemRole}}},
            teams: {
                currentTeamId: 't1',
                teams: {t1: {id: 't1', name: 'lab', delete_at: 0}, t2: {id: 't2', name: 'ops', delete_at: 0}},
                myMembers: {t1: {team_id: 't1', user_id: 'me', roles: 'team_user'}, t2: {team_id: 't2', user_id: 'me', roles: 'team_admin'}},
            },
            channels: {myMembers: {}, channels: {}, roles: {}},
            roles: {
                roles: {
                    [systemRole]: {id: 'r1', name: systemRole, permissions: system},
                    team_user: {id: 'r2', name: 'team_user', permissions: []},
                    team_admin: {id: 'r3', name: 'team_admin', permissions: team},
                },
            },
        },
    } as unknown as GlobalState;
}

const rights = (s: GlobalState) => ({
    create: canCreateCustomEmoji(s),
    deleteOwn: canDeleteOwnCustomEmoji(s),
    deleteOthers: canDeleteOthersCustomEmoji(s),
    manage: canManageCustomEmoji(s),
});

describe('fusion/selectors/custom_emoji', () => {
    test('members add emoji and delete their own (the default system_user role)', () => {
        expect(rights(state({system: ['create_emojis', 'delete_emojis']}))).toEqual({create: true, deleteOwn: true, deleteOthers: false, manage: true});
    });

    test('admins delete anyone\'s', () => {
        expect(rights(state({systemRole: 'system_admin', system: ['create_emojis', 'delete_emojis', 'delete_others_emojis']}))).toEqual({create: true, deleteOwn: true, deleteOthers: true, manage: true});
    });

    test('guests have none of it (the default system_guest role)', () => {
        expect(rights(state({systemRole: 'system_guest'}))).toEqual({create: false, deleteOwn: false, deleteOthers: false, manage: false});
    });

    test('a permission from the role in any community counts, as on the server', () => {
        expect(rights(state({team: ['create_emojis']}))).toMatchObject({create: true, manage: true});
        expect(rights(state({system: ['delete_emojis'], team: ['delete_others_emojis']}))).toMatchObject({deleteOwn: true, deleteOthers: true});
    });

    test('delete_others_emojis alone deletes nothing', () => {
        expect(rights(state({system: ['delete_others_emojis']}))).toEqual({create: false, deleteOwn: false, deleteOthers: false, manage: false});
    });

    test('with custom emoji off, nobody adds or deletes, admins included', () => {
        expect(rights(state({enabled: false, systemRole: 'system_admin', system: ['create_emojis', 'delete_emojis', 'delete_others_emojis']}))).toEqual({create: false, deleteOwn: false, deleteOthers: false, manage: false});
    });
});
