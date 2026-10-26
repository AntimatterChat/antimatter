// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {renderHookWithContext} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {useLastMessage} from './last_message';

// The home list's second line is a conversation's last message. sec-fusion #5: the text of someone you ignore or block
// showed there, although their messages fold in the conversation.
describe('fusion/sidebar/useLastMessage', () => {
    const channel = TestHelper.getChannelMock({id: 'c1', type: 'O', team_id: 't1', last_post_at: 100});

    const stateWith = (userId: string, prefs: Record<string, unknown> = {}, props: Record<string, unknown> = {}) => ({
        entities: {
            users: {currentUserId: 'me', profiles: {}},
            general: {config: {}},
            preferences: {myPreferences: prefs},
            channels: {channels: {c1: channel}},
            posts: {
                posts: {p1: TestHelper.getPostMock({id: 'p1', channel_id: 'c1', user_id: userId, message: 'secret plans', create_at: 100, props})},
                postsInChannel: {c1: [{order: ['p1'], recent: true, oldest: false}]},
            },
        },
    } as never);

    test('shows the last message', () => {
        const {result} = renderHookWithContext(() => useLastMessage(channel), stateWith('bob'));
        expect(result.current).toBe('secret plans');
    });

    test('shows no text from someone you ignore', () => {
        const {result} = renderHookWithContext(() => useLastMessage(channel), stateWith('eve', {'user_block--eve': {category: 'user_block', name: 'eve', value: 'ignore', user_id: 'me'}}));
        expect(result.current).toBe('');
    });

    test('shows no text from someone you block', () => {
        const {result} = renderHookWithContext(() => useLastMessage(channel), stateWith('eve', {'user_block--eve': {category: 'user_block', name: 'eve', value: 'block', user_id: 'me'}}));
        expect(result.current).toBe('');
    });

    test('still says Spoiler for a spoiler', () => {
        const {result} = renderHookWithContext(() => useLastMessage(channel), stateWith('bob', {}, {spoiler: true}));
        expect(result.current).toBe('Spoiler');
    });
});
