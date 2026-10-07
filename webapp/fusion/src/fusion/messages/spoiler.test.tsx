// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import Message from './message';
import {isSpoiler, setSpoilerShown} from './spoiler';

describe('fusion/messages/spoiler', () => {
    const me = TestHelper.getUserMock({id: 'me', username: 'jason'});
    const author = TestHelper.getUserMock({id: 'author', username: 'marie'});
    const spoiler = TestHelper.getPostMock({id: 'spoiler', type: '', channel_id: 'channel', user_id: author.id, message: 'The butler did it', props: {spoiler: true}});

    const render = () => renderWithContext(
        <Message postId={spoiler.id}/>,
        {
            entities: {
                users: {currentUserId: me.id, profiles: {me, author}},
                posts: {posts: {[spoiler.id]: spoiler}},
                channels: {channels: {channel: TestHelper.getChannelMock({id: 'channel', team_id: 'team'})}},
                teams: {currentTeamId: 'team', teams: {team: TestHelper.getTeamMock({id: 'team', name: 'team'})}},
            },
        } as never,
    );

    afterEach(() => setSpoilerShown(spoiler.id, false));

    test('tells spoilers from other messages', () => {
        expect(isSpoiler(spoiler)).toBe(true);
        expect(isSpoiler({props: {spoiler: 'true'}})).toBe(true);
        expect(isSpoiler({props: {}})).toBe(false);
        expect(isSpoiler(undefined)).toBe(false);
    });

    test('hides the text until clicked, then hides it again from the tag', async () => {
        render();

        // The text stays in place, at its size, but can't be read out or reached until shown.
        const veil = screen.getByRole('button', {name: 'Spoiler, click to show it'});
        expect(veil.firstElementChild).toHaveAttribute('aria-hidden', 'true');
        expect(screen.queryByRole('button', {name: 'Hide the spoiler again'})).not.toBeInTheDocument();

        await userEvent.click(veil);
        expect(screen.queryByRole('button', {name: 'Spoiler, click to show it'})).not.toBeInTheDocument();
        expect(screen.getByText('The butler did it')).toBeVisible();

        await userEvent.click(screen.getByRole('button', {name: 'Hide the spoiler again'}));
        expect(screen.getByRole('button', {name: 'Spoiler, click to show it'})).toBeInTheDocument();
    });

    test('stays shown when the message is drawn again', async () => {
        const {unmount} = render();
        await userEvent.click(screen.getByRole('button', {name: 'Spoiler, click to show it'}));
        unmount();

        render();
        expect(screen.queryByRole('button', {name: 'Spoiler, click to show it'})).not.toBeInTheDocument();
    });
});
