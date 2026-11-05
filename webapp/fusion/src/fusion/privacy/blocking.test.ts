// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ChannelMembership} from '@mattermost/types/channels';

import {deletePreferences} from 'mattermost-redux/actions/preferences';
import {Client4} from 'mattermost-redux/client';
import {calculateUnreadCount, setMessageRequestsEnabled} from 'mattermost-redux/utils/channel_utils';

import {TestHelper} from 'utils/test_helper';

import type {GlobalState} from 'types/store';

import {getBlockLevel, getBlockLists, getHiddenDirectIds, getMessageRequests, setBlockLevel} from './blocking';

jest.mock('mattermost-redux/actions/preferences', () => ({
    savePreferences: jest.fn(() => ({type: 'SAVE_PREFERENCES'})),
    deletePreferences: jest.fn(() => ({type: 'DELETE_PREFERENCES'})),
}));

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

    // audit-1104 fusion-core-5: stopping ignoring or blocking someone accepted whatever request their direct message
    // was, also a stranger's that was waiting before, round the person's "Ask me first".
    describe('setBlockLevel', () => {
        const blockState = (request: 'pending' | 'dismissed' | 'accepted', {creator = 'stranger', privacy = 'ask'} = {}) => ({
            entities: {
                general: {config: {EnableUserBlocking: 'true'}},
                users: {currentUserId: me, profiles: {}},
                channels: {
                    channels: {'dm-stranger': {...dm('stranger'), creator_id: creator}},
                    myMembers: {'dm-stranger': member('dm-stranger', request)},
                },
                preferences: {
                    myPreferences: {
                        'user_block--stranger': pref('stranger', 'ignore'),
                        ...(privacy ? {'dm_privacy--default': {user_id: me, category: 'dm_privacy', name: 'default', value: privacy}} : {}),
                    },
                },
            },
        } as unknown as GlobalState);

        const run = async (state: GlobalState, level: '' | 'ignore' | 'block') => {
            const getState = () => state;
            const dispatch: jest.Mock = jest.fn((action: unknown) => (typeof action === 'function' ? action(dispatch, getState, undefined) : {data: true}));
            return setBlockLevel('stranger', level)(dispatch as never, getState as never, undefined as never);
        };

        let updateMessageRequest: jest.SpyInstance;
        beforeEach(() => {
            jest.clearAllMocks();
            updateMessageRequest = jest.spyOn(Client4, 'updateMessageRequest').mockImplementation((channelId) => Promise.resolve(member(channelId, 'accepted')));
        });
        afterEach(() => updateMessageRequest.mockRestore());

        test('a stranger\'s request waiting before they were ignored stays a request when you ask first', async () => {
            await run(blockState('pending'), '');

            expect(deletePreferences).toHaveBeenCalled();
            expect(updateMessageRequest).not.toHaveBeenCalled();
        });

        test('a request you ignored stays ignored, whatever your choice for direct messages', async () => {
            await run(blockState('dismissed', {privacy: 'everyone'}), '');
            await run(blockState('dismissed', {creator: me}), '');

            expect(updateMessageRequest).not.toHaveBeenCalled();
        });

        test('a conversation you started goes back among your conversations', async () => {
            await run(blockState('pending', {creator: me}), '');

            expect(updateMessageRequest).toHaveBeenCalledWith('dm-stranger', 'accept');
        });

        test('it goes back too when you take direct messages from everyone: accepting gets round no choice', async () => {
            await run(blockState('pending', {privacy: ''}), '');
            await run(blockState('pending', {privacy: 'everyone'}), '');

            expect(updateMessageRequest).toHaveBeenCalledTimes(2);
            expect(updateMessageRequest).toHaveBeenCalledWith('dm-stranger', 'accept');
        });

        test('ignoring someone puts your conversation with them among the requests', async () => {
            await run(blockState('accepted'), 'ignore');

            expect(updateMessageRequest).toHaveBeenCalledWith('dm-stranger', 'request');
        });
    });
});
