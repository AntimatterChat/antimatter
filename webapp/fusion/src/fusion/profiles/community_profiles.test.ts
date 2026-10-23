// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import type {CommunityProfile} from '@mattermost/types/community_profiles';

import {TeamTypes} from 'mattermost-redux/action_types';
import {communityProfiles} from 'mattermost-redux/reducers/entities/teams';

import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import {avatarURLIn, communityView, displayNameIn, getCurrentScope, getNameDisplayIn, globalNameOf, scopeOfChannel} from './community_profiles';

const maya = TestHelper.getUserMock({id: 'maya', username: 'maya.l', first_name: 'Maya', last_name: 'Lumen', nickname: 'Mayo', last_picture_update: 5});
const captain: CommunityProfile = {team_id: 't1', user_id: 'maya', nickname: 'Captain', update_at: 1};

// A state with two communities: t1, whose admins chose to show nicknames, and t2, which didn't choose (the server
// shows full names by default); the user's own choice (for direct messages) is usernames.
function state({currentChannel = '', profiles = {t1: {maya: captain}} as Record<string, Record<string, CommunityProfile>>} = {}): GlobalState {
    return {
        entities: {
            general: {config: {TeammateNameDisplay: 'full_name'}},
            preferences: {myPreferences: {'display_settings--name_format': {user_id: 'me', category: 'display_settings', name: 'name_format', value: 'username'}}},
            teams: {
                currentTeamId: 't1',
                teams: {
                    t1: TestHelper.getTeamMock({id: 't1', name_display: 'nickname_full_name'}),
                    t2: TestHelper.getTeamMock({id: 't2', name_display: ''}),
                },
                communityProfiles: profiles,
            },
            channels: {
                currentChannelId: currentChannel,
                channels: {
                    c1: TestHelper.getChannelMock({id: 'c1', team_id: 't1', type: 'O'}),
                    dm: TestHelper.getChannelMock({id: 'dm', team_id: '', type: 'D'}),
                    gm: TestHelper.getChannelMock({id: 'gm', team_id: '', type: 'G'}),
                },
            },
        },
    } as unknown as GlobalState;
}

describe('fusion/profiles/community_profiles', () => {
    test('a channel\'s scope is its community; direct and group messages have the global profile', () => {
        const s = state();
        expect(scopeOfChannel(s.entities.channels.channels.c1)).toBe('t1');
        expect(scopeOfChannel(s.entities.channels.channels.dm)).toBe('');
        expect(scopeOfChannel(s.entities.channels.channels.gm)).toBe('');
        expect(scopeOfChannel(undefined)).toBeUndefined();
    });

    test('the current scope follows the conversation open, else the current community', () => {
        expect(getCurrentScope(state({currentChannel: 'c1'}))).toBe('t1');
        expect(getCurrentScope(state({currentChannel: 'dm'}))).toBe('');
        expect(getCurrentScope(state())).toBe('t1');
    });

    test('a community shows people as its admins chose, else as the server does; outside, as the user chose', () => {
        const s = state();
        expect(getNameDisplayIn(s, 't1')).toBe('nickname_full_name');
        expect(getNameDisplayIn(s, 't2')).toBe('full_name');
        expect(getNameDisplayIn(s, '')).toBe('username');
    });

    test('a community shows someone with their nickname there; elsewhere their global profile shows', () => {
        const s = state();
        expect(displayNameIn(s, maya, 't1')).toBe('Captain');
        expect(displayNameIn(s, maya, 't2')).toBe('Maya Lumen');
        expect(displayNameIn(s, maya, '')).toBe('maya.l');
        expect(globalNameOf(s, maya)).toBe('maya.l');
        expect(communityView(maya, undefined)).toBe(maya);
        expect(communityView(maya, captain).nickname).toBe('Captain');
        expect(maya.nickname).toBe('Mayo');
    });

    test('names the server took off (privacy) leave the username, or the nickname chosen for the community', () => {
        const blank = {...maya, first_name: '', last_name: '', nickname: ''};
        expect(displayNameIn(state(), blank, 't2')).toBe('maya.l');
        expect(displayNameIn(state(), blank, 't1')).toBe('Captain');
    });

    test('a picture of their own in the community, else their profile picture', () => {
        expect(avatarURLIn(maya, {...captain, avatar_update_at: 42}, 't1')).toMatch(/\/teams\/t1\/members\/maya\/profile\/image\?_=42$/);
        expect(avatarURLIn(maya, captain, 't1')).toMatch(/\/users\/maya\/image\?_=5$/);
        expect(avatarURLIn(maya, {...captain, avatar_update_at: 42}, '')).toMatch(/\/users\/maya\/image\?_=5$/);
    });
});

describe('the communityProfiles reducer', () => {
    test('keeps each community\'s profiles by user, and each change the server sends', () => {
        let s = communityProfiles({}, {type: TeamTypes.RECEIVED_COMMUNITY_PROFILES, data: {teamId: 't1', profiles: [captain]}});
        expect(s.t1.maya.nickname).toBe('Captain');

        s = communityProfiles(s, {type: TeamTypes.RECEIVED_COMMUNITY_PROFILE, data: {team_id: 't1', user_id: 'sam', avatar_update_at: 3, update_at: 3}});
        expect(Object.keys(s.t1).sort()).toEqual(['maya', 'sam']);

        // An empty profile is gone.
        s = communityProfiles(s, {type: TeamTypes.RECEIVED_COMMUNITY_PROFILE, data: {team_id: 't1', user_id: 'maya', update_at: 4}});
        expect(Object.keys(s.t1)).toEqual(['sam']);

        // Communities not loaded yet load theirs when opened.
        expect(communityProfiles(s, {type: TeamTypes.RECEIVED_COMMUNITY_PROFILE, data: {team_id: 't9', user_id: 'x', nickname: 'X', update_at: 1}})).toBe(s);
    });
});
