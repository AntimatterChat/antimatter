// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Channel} from '@mattermost/types/channels';

import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import {getDefaultChannel, isAllChannelsCommunity, isCommunityChannelFor, isDefaultChannel, isFirstDefaultChannel} from './community_channels';

const ch = (id: string, name: string, extra: Partial<Channel> = {}) => TestHelper.getChannelMock({id, name, display_name: name, team_id: 't1', type: 'O', delete_at: 0, ...extra});
const town = ch('town', 'general');
const offTopic = ch('off', 'off-topic');
const dev = ch('dev', 'dev');
const secret = ch('secret', 'secret', {type: 'P'});
const gated = ch('gated', 'gated', {type: 'P', group_constrained: true});
const old = ch('old', 'old', {delete_at: 5});

function state({allChannels = true, defaultChannelId = '', channels = [town, offTopic, dev, secret, gated, old]}: {allChannels?: boolean; defaultChannelId?: string; channels?: Channel[]} = {}) {
    return {
        entities: {
            teams: {teams: {t1: TestHelper.getTeamMock({id: 't1', all_channels_membership: allChannels, default_channel_id: defaultChannelId})}},
            channels: {
                channels: Object.fromEntries(channels.map((c) => [c.id, c])),
                channelsInTeam: {t1: new Set(channels.map((c) => c.id))},
            },
            users: {
                profiles: {
                    ana: TestHelper.getUserMock({id: 'ana', roles: 'system_user', delete_at: 0}),
                    guest: TestHelper.getUserMock({id: 'guest', roles: 'system_guest', delete_at: 0}),
                    bot: TestHelper.getUserMock({id: 'bot', roles: 'system_user', is_bot: true, delete_at: 0}),
                    gone: TestHelper.getUserMock({id: 'gone', roles: 'system_user', delete_at: 9}),
                },
            },
        },
    } as unknown as GlobalState;
}

describe('fusion/channel/community_channels', () => {
    test('a community has everyone in every channel only when it says so', () => {
        expect(isAllChannelsCommunity(state(), 't1')).toBe(true);
        expect(isAllChannelsCommunity(state({allChannels: false}), 't1')).toBe(false);
        expect(isAllChannelsCommunity(state(), 'unknown')).toBe(false);
    });

    test('members stay in the public channels and the group-synced private ones of an all-channels community', () => {
        expect(isCommunityChannelFor(state(), dev, 'ana')).toBe(true);
        expect(isCommunityChannelFor(state(), gated, 'ana')).toBe(true);

        // Other private channels are joined by invitation, and can be left.
        expect(isCommunityChannelFor(state(), secret, 'ana')).toBe(false);
    });

    test('guests, bots and deactivated people are only where they were added', () => {
        expect(isCommunityChannelFor(state(), dev, 'guest')).toBe(false);
        expect(isCommunityChannelFor(state(), dev, 'bot')).toBe(false);
        expect(isCommunityChannelFor(state(), dev, 'gone')).toBe(false);
        expect(isCommunityChannelFor(state(), dev, 'unknown')).toBe(false);
    });

    test('nobody is held in archived or policy-enforced channels, or in a community without the setting', () => {
        expect(isCommunityChannelFor(state(), old, 'ana')).toBe(false);
        expect(isCommunityChannelFor(state(), {...dev, policy_enforced: true}, 'ana')).toBe(false);
        expect(isCommunityChannelFor(state({allChannels: false}), dev, 'ana')).toBe(false);
        expect(isCommunityChannelFor(state(), undefined, 'ana')).toBe(false);
    });

    test('the default channel is General unless the community chose another', () => {
        expect(getDefaultChannel(state(), 't1')?.id).toBe('town');
        expect(isDefaultChannel(state(), town)).toBe(true);
        expect(isDefaultChannel(state(), dev)).toBe(false);

        const chosen = state({defaultChannelId: 'dev'});
        expect(getDefaultChannel(chosen, 't1')?.id).toBe('dev');
        expect(isDefaultChannel(chosen, dev)).toBe(true);
        expect(isDefaultChannel(chosen, town)).toBe(false);
    });

    test('an archived or private default channel leaves the community without one', () => {
        expect(getDefaultChannel(state({defaultChannelId: 'old'}), 't1')).toBeUndefined();
        expect(getDefaultChannel(state({defaultChannelId: 'secret'}), 't1')).toBeUndefined();
        expect(getDefaultChannel(state({channels: [{...town, delete_at: 1}, dev]}), 't1')).toBeUndefined();
        expect(getDefaultChannel(state({channels: [dev]}), 't1')).toBeUndefined();
        expect(getDefaultChannel(state(), 'unknown')).toBeUndefined();
    });

    test('General, Welcome, Off-Topic and the chosen default are the channels only admins archive', () => {
        const chosen = state({defaultChannelId: 'dev'});
        expect(isFirstDefaultChannel(chosen, town)).toBe(true);
        expect(isFirstDefaultChannel(chosen, {...dev, id: 'welcome', name: 'welcome'})).toBe(true);
        expect(isFirstDefaultChannel(chosen, offTopic)).toBe(true);
        expect(isFirstDefaultChannel(chosen, dev)).toBe(true);
        expect(isFirstDefaultChannel(state(), dev)).toBe(false);
    });
});
