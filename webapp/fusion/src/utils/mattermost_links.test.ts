// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {blockMattermostLinks, isMattermostURL} from './mattermost_links';

describe('mattermost links', () => {
    test.each([
        ['https://mattermost.com/pl/default-docs', true],
        ['https://docs.mattermost.com/guide', true],
        ['http://www.mattermost.com', true],
        ['https://notmattermost.com', false],
        ['https://github.com/AntimatterChat', false],
        ['/help/formatting', false],
        ['', false],
    ])('%s goes to mattermost.com: %s', (url, expected) => {
        expect(isMattermostURL(url)).toBe(expected);
    });

    test('refuses to open them, and to follow their links', () => {
        const open = jest.fn(() => null);
        window.open = open;
        const undo = blockMattermostLinks();

        window.open('https://docs.mattermost.com', '_blank');
        window.open('https://example.com', '_blank');
        expect(open).toHaveBeenCalledTimes(1);
        expect(open).toHaveBeenCalledWith('https://example.com', '_blank');

        const link = document.createElement('a');
        link.href = 'https://mattermost.com/download';
        document.body.appendChild(link);
        const click = new MouseEvent('click', {bubbles: true, cancelable: true});
        link.dispatchEvent(click);
        expect(click.defaultPrevented).toBe(true);

        undo();
        link.remove();
        expect(window.open).toBe(open);
    });
});
