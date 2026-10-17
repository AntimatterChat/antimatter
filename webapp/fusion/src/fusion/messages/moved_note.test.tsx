// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {fireEvent, renderWithContext, screen} from 'tests/react_testing_utils';
import {getHistory} from 'utils/browser_history';
import {TestHelper} from 'utils/test_helper';

import Message from './message';
import MovedNote from './moved_note';

const me = TestHelper.getUserMock({id: 'me', username: 'maya', first_name: 'Maya', roles: 'system_user'});
const tom = TestHelper.getUserMock({id: 'tom', username: 'tom', first_name: 'Tom', last_name: 'Okafor'});
const team = TestHelper.getTeamMock({id: 'team', name: 'lumen'});
const general = TestHelper.getChannelMock({id: 'general', name: 'general', display_name: 'General', type: 'O', team_id: 'team'});
const oncall = TestHelper.getChannelMock({id: 'oncall', name: 'ops-oncall', display_name: 'Ops on call', type: 'O', team_id: 'team'});

const link = 'https://antimatter.example.com/lumen/pl/newroot';
const pointer = TestHelper.getPostMock({
    id: 'pointer',
    type: 'system_wrangler',
    channel_id: 'general',
    user_id: 'me',
    message: `A thread with 13 messages has been moved to [#ops-oncall](${link})`,
    props: {
        TranslationID: 'app.post.move_thread.moved_to.multiple',
        MovedThreadPermalink: link,
        NumMessages: 13,
        MovedThreadChannelId: 'oncall',
        MovedThreadChannelName: 'ops-oncall',
        MovedThreadChannelDisplayName: 'Ops on call',
        MovedThreadTeamName: 'lumen',
        MovedThreadPostId: 'newroot',
    },
    metadata: {embeds: [{type: 'permalink', url: '', data: {post_id: 'newroot'}}]} as never,
});
const note = TestHelper.getPostMock({
    id: 'note',
    type: 'system_wrangler',
    channel_id: 'oncall',
    user_id: 'me',
    message: 'This thread was moved from [#general](https://antimatter.example.com/lumen/pl/pointer)',
    props: {
        TranslationID: 'app.post.move_thread.moved_from',
        MovedFromChannelId: 'general',
        MovedFromChannelName: 'general',
        MovedFromChannelDisplayName: 'General',
        MovedFromTeamName: 'lumen',
        MovedFromPostId: 'pointer',
        MovedFromPermalink: 'https://antimatter.example.com/lumen/pl/pointer',
    },
});

function state(channels: Record<string, unknown> = {general, oncall}, posts: Array<ReturnType<typeof TestHelper.getPostMock>> = []) {
    return {
        entities: {
            general: {config: {}},
            users: {currentUserId: 'me', profiles: {me, tom}},
            teams: {currentTeamId: 'team', teams: {team}, myMembers: {team: {team_id: 'team', user_id: 'me', roles: 'team_user'}}},
            channels: {channels, myMembers: {general: {channel_id: 'general', user_id: 'me', roles: 'channel_user'}}, channelsInTeam: {}},
            posts: {posts: Object.fromEntries(posts.map((p) => [p.id, p])), postsInThread: {}, reactions: {}},
            preferences: {myPreferences: {}},
        },
    } as never;
}

describe('fusion/messages/MovedNote', () => {
    test('the pointer names the channel as a link to the moved thread, not a raw address', () => {
        const {container} = renderWithContext(<MovedNote post={pointer}/>, state());
        expect(container).toHaveTextContent('A thread with 13 messages has been moved to #Ops on call');
        expect(container).not.toHaveTextContent('https://');
        const anchor = screen.getByRole('link', {name: '#Ops on call'});
        expect(anchor).toHaveClass('mention-link');
        expect(anchor).toHaveAttribute('href', '/lumen/pl/newroot');

        const push = jest.spyOn(getHistory(), 'push').mockImplementation(() => undefined);
        anchor.click();
        expect(push).toHaveBeenCalledWith('/lumen/pl/newroot');
        push.mockRestore();
    });

    test('the link opens the moved thread in the app, as links to messages do: no page load', () => {
        renderWithContext(<MovedNote post={pointer}/>, state());
        const push = jest.spyOn(getHistory(), 'push').mockImplementation(() => undefined);

        // fireEvent.click is false when the click's default (following the address, a page load) is prevented.
        expect(fireEvent.click(screen.getByRole('link', {name: '#Ops on call'}))).toBe(false);
        expect(push).toHaveBeenCalledTimes(1);
        expect(push).toHaveBeenCalledWith('/lumen/pl/newroot');
        push.mockRestore();
    });

    test('the note\'s link opens the pointer left in the old channel, in the app', () => {
        renderWithContext(<MovedNote post={note}/>, state());
        const push = jest.spyOn(getHistory(), 'push').mockImplementation(() => undefined);
        expect(fireEvent.click(screen.getByRole('link', {name: '#General'}))).toBe(false);
        expect(push).toHaveBeenCalledWith('/lumen/pl/pointer');
        push.mockRestore();
    });

    test('a click with a modifier is left to the browser: a new tab or window', () => {
        renderWithContext(<MovedNote post={pointer}/>, state());
        const push = jest.spyOn(getHistory(), 'push').mockImplementation(() => undefined);
        const anchor = screen.getByRole('link', {name: '#Ops on call'});
        expect(fireEvent.click(anchor, {ctrlKey: true})).toBe(true);
        expect(fireEvent.click(anchor, {metaKey: true})).toBe(true);
        expect(fireEvent.click(anchor, {shiftKey: true})).toBe(true);
        expect(push).not.toHaveBeenCalled();
        push.mockRestore();
    });

    test('one message', () => {
        const {container} = renderWithContext(<MovedNote post={{...pointer, props: {...pointer.props, NumMessages: 1}}}/>, state());
        expect(container).toHaveTextContent('A message has been moved to #Ops on call');
    });

    test('the note names where the thread came from, linking to the pointer left there', () => {
        const {container} = renderWithContext(<MovedNote post={note}/>, state());
        expect(container).toHaveTextContent('This thread was moved from #General');
        expect(container).not.toHaveTextContent('https://');
        expect(screen.getByRole('link', {name: '#General'})).toHaveAttribute('href', '/lumen/pl/pointer');
    });

    test('a private channel the reader isn\'t in is "another channel", still a link', () => {
        const hidden = {...pointer, props: {...pointer.props, MovedThreadChannelId: 'secret', MovedThreadChannelName: undefined, MovedThreadChannelDisplayName: undefined}};
        const {container} = renderWithContext(<MovedNote post={hidden}/>, state());
        expect(container).toHaveTextContent('A thread with 13 messages has been moved to another channel');
        expect(screen.getByRole('link', {name: 'another channel'})).toHaveAttribute('href', '/lumen/pl/newroot');
    });

    test('a private channel the reader is in shows its name, from what they know', () => {
        const secret = TestHelper.getChannelMock({id: 'secret', name: 'secret', display_name: 'Secret plans', type: 'P', team_id: 'team'});
        const hidden = {...pointer, props: {...pointer.props, MovedThreadChannelId: 'secret', MovedThreadChannelName: undefined, MovedThreadChannelDisplayName: undefined}};
        renderWithContext(<MovedNote post={hidden}/>, state({general, oncall, secret}));
        expect(screen.getByRole('link', {name: '#Secret plans'})).toBeInTheDocument();
    });

    test('an older pointer, without the channel, keeps its old text', () => {
        const old = TestHelper.getPostMock({
            id: 'old',
            type: 'system_wrangler',
            message: 'A thread with 5 messages has been moved: https://antimatter.example.com/lumen/pl/x\n',
            props: {TranslationID: 'app.post.move_thread_command.channel.multiple_messages', MovedThreadPermalink: 'https://antimatter.example.com/lumen/pl/x', NumMessages: 5},
        });
        const {container} = renderWithContext(<MovedNote post={old}/>, state());
        expect(container).toHaveTextContent('has been moved');
    });
});

describe('fusion/messages/Message with moved messages', () => {
    test('the pointer and the note are one-line system messages, without a preview of the linked message', () => {
        const {container} = renderWithContext(
            <>
                <Message postId='pointer'/>
                <Message postId='note'/>
            </>,
            state(undefined, [pointer, note]),
        );
        const lines = container.querySelectorAll('.am-sysline');
        expect(lines).toHaveLength(2);
        expect(container.querySelector('.am-msg-head')).toBeNull();
        expect(container.querySelector('.post-preview, .permalink, .attachment--permalink')).toBeNull();
        expect(lines[0]).toHaveTextContent('A thread with 13 messages has been moved to #Ops on call');
        expect(lines[1]).toHaveTextContent('This thread was moved from #General');
    });

    test('a moved copy is a normal message: one header row with the author and the time', () => {
        const copy = TestHelper.getPostMock({
            id: 'copy',
            type: '',
            channel_id: 'oncall',
            user_id: 'tom',
            root_id: '',
            message: 'Rota for November is in the sheet.',
            create_at: Date.UTC(2026, 9, 7, 19, 30),
            edit_at: 0,
            props: {silent_notification: true},
        });
        const {container} = renderWithContext(<Message postId='copy'/>, state(undefined, [copy]));
        const heads = container.querySelectorAll('.am-msg-head');
        expect(heads).toHaveLength(1);
        expect(heads[0].querySelector('.am-author')).toBeInTheDocument();
        expect(heads[0].querySelector('.am-time')).toBeInTheDocument();
        expect(container.querySelector('.am-sysline')).toBeNull();
        expect(container).not.toHaveTextContent(/edited/i);
    });
});
