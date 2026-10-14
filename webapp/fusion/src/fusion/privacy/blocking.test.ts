// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ChannelMembership} from '@mattermost/types/channels';

import {calculateUnreadCount, setMessageRequestsEnabled} from 'mattermost-redux/utils/channel_utils';

import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import {getBlockLevel, getBlockLists, getHiddenDirectIds, getMessageRequests} from './blocking';

describe('fusion/privacy/blocking', () => {
    const me = 'me';
    const dm = (other: string, lastPostAt = 1) => TestHelper.getChannelMock({id: `dm-${other}`, name: [me, other].sort().join('__'), type: 'D', team_id: '', last_post_at: lastPostAt});
    const member = (channelId: string, request?: 'pending' | 'dismissed' | 'accepted') => TestHelper.getChannelMembershipMock({
        channel_id: channelId,
        user_id: me,
        msg_count: 0,
        mention_count: 2,
        notify_props: {...TestHelper.getChannelMembershipMock({}).notify_props, ...(request ? {dm_request: request} : {})} as ChannelMembership['notify_props'],
    });
    const pref = (name: string, value: string) => ({user_id: me, category: 'user_block', name, value});

    const channels = {
        'dm-alice': dm('alice'),
        'dm-bob': dm('bob', 2),
        'dm-carol': dm('carol', 3),
        'dm-dave': dm('dave'),
        'dm-erin': dm('erin', 0),
    };
    const state = (enabled = true) => ({
        entities: {
            general: {config: {EnableUserBlocking: String(enabled)}},
            users: {
                currentUserId: me,
                profiles: Object.fromEntries([me, 'alice', 'bob', 'carol', 'dave', 'erin'].map((id) => [id, TestHelper.getUserMock({id, username: id})])),
                profilesInChannel: {},
            },
            channels: {
                channels,
                messageCounts: {
                    'dm-alice': {total: 3, root: 3},
                    'dm-bob': {total: 3, root: 3},
                    'dm-carol': {total: 3, root: 3},
                    'dm-dave': {total: 3, root: 3},
                    'dm-erin': {total: 0, root: 0},
                },
                myMembers: {
                    'dm-alice': member('dm-alice', 'accepted'),
                    'dm-bob': member('dm-bob', 'pending'),
                    'dm-carol': member('dm-carol', 'dismissed'),
                    'dm-dave': member('dm-dave', 'pending'),
                    'dm-erin': member('dm-erin', 'pending'),
                },
            },
            preferences: {
                myPreferences: {
                    'user_block--dave': pref('dave', 'block'),
                    'user_block--alice': pref('alice', 'ignore'),
                    'user_block--zed': pref('zed', 'mute'),
                },
            },
        },
    } as unknown as GlobalState);

    beforeEach(() => setMessageRequestsEnabled(true));
    afterAll(() => setMessageRequestsEnabled(false));

    test('lists whom you block and ignore', () => {
        expect(getBlockLists(state())).toEqual({blocked: ['dave'], ignored: ['alice']});
        expect(getBlockLevel(state(), 'dave')).toBe('block');
        expect(getBlockLevel(state(), 'zed')).toBe('');
        expect(getBlockLevel(state(false), 'dave')).toBe('');
    });

    test('lists the pending requests with messages apart from those ignored, leaving out people blocked', () => {
        const {pending, dismissed} = getMessageRequests(state());
        expect(pending.map((r) => r.userId)).toEqual(['bob']);
        expect(dismissed.map((r) => r.userId)).toEqual(['carol']);
        expect([...getHiddenDirectIds(state())].sort()).toEqual(['dm-bob', 'dm-carol', 'dm-dave', 'dm-erin']);
    });

    test('nothing is a request while the admin turned it off', () => {
        expect(getMessageRequests(state(false))).toEqual({pending: [], dismissed: []});
    });

    test('requests are left out of the unread counts while message requests are on', () => {
        const count = {total: 3, root: 3};
        expect(calculateUnreadCount(count, member('dm-bob', 'pending'), false)).toMatchObject({showUnread: false, mentions: 0, messages: 0});
        expect(calculateUnreadCount(count, member('dm-alice', 'accepted'), false)).toMatchObject({showUnread: true, mentions: 2, messages: 3});

        setMessageRequestsEnabled(false);
        expect(calculateUnreadCount(count, member('dm-bob', 'pending'), false)).toMatchObject({showUnread: true, mentions: 2});
    });
});
