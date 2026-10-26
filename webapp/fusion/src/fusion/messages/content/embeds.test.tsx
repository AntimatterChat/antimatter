// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {imageNameOf, InlineImage} from './embeds';

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
