// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {imageNameOf, InlineImage, Permalink} from './embeds';

// A linked image is named after the last segment of its address. A malformed escape in it used to throw while the
// message was drawn (decodeURIComponent's URIError), which took the whole app down for everyone in the channel.
describe('fusion/messages/content/embeds: InlineImage', () => {
    const state = {
        entities: {
            general: {config: {HasImageProxy: 'false', EnableSVGs: 'true'}},
            users: {currentUserId: 'me', profiles: {}},
        },
    } as never;

    test('names an image with a malformed escape as written, without throwing', () => {
        const link = 'https://x.test/a%E0%A4.png';
        const post = TestHelper.getPostMock({id: 'p1', message: link, metadata: {images: {[link]: {width: 10, height: 10}}} as never});
        expect(() => renderWithContext(
            <InlineImage
                post={post}
                link={link}
            />,
            state,
        )).not.toThrow();
        expect(screen.getByRole('button', {name: 'Open image a%E0%A4.png'})).toBeInTheDocument();
        expect(screen.getByText(/a%E0%A4\.png/)).toBeInTheDocument();
    });

    test('decodes a well-formed name', () => {
        expect(imageNameOf('https://x.test/dir/my%20cat.png?x=1#y')).toBe('my cat.png');
    });

    test('keeps a malformed name as written', () => {
        expect(imageNameOf('https://x.test/a%E0%A4.png')).toBe('a%E0%A4.png');
    });

    test('falls back to the whole link when it has no name', () => {
        expect(imageNameOf('https://x.test/')).toBe('https://x.test/');
    });
});

// sec-fusion #5: a link to a spoiler, or to a message from someone you ignore, previewed its text.
describe('fusion/messages/content/embeds: Permalink', () => {
    const state = {
        entities: {
            general: {config: {}},
            users: {currentUserId: 'me', profiles: {bob: TestHelper.getUserMock({id: 'bob', username: 'bob'}), eve: TestHelper.getUserMock({id: 'eve', username: 'eve'})}},
            teams: {currentTeamId: 't1', teams: {t1: TestHelper.getTeamMock({id: 't1', name: 'lumen'})}},
            preferences: {myPreferences: {'user_block--eve': {category: 'user_block', name: 'eve', value: 'ignore', user_id: 'me'}}},
        },
    } as never;
    const data = (post: ReturnType<typeof TestHelper.getPostMock>) => ({post_id: post.id, post, channel_display_name: 'town', channel_type: 'O', team_name: 'lumen', channel_id: 'c1'}) as never;

    test('a linked spoiler keeps its text hidden', () => {
        renderWithContext(<Permalink data={data(TestHelper.getPostMock({id: 'p1', user_id: 'bob', message: 'the twist ending', props: {spoiler: true}}))}/>, state);
        expect(screen.getByText('Spoiler')).toBeInTheDocument();
        expect(document.body.textContent).not.toContain('the twist ending');
    });

    test('a linked message from someone you ignore keeps its text hidden', () => {
        renderWithContext(<Permalink data={data(TestHelper.getPostMock({id: 'p2', user_id: 'eve', message: 'hello from eve'}))}/>, state);
        expect(screen.getByText('Message from someone you ignore')).toBeInTheDocument();
        expect(document.body.textContent).not.toContain('hello from eve');
    });
});
