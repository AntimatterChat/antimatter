// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {act, render} from '@testing-library/react';
import React from 'react';

import PluggableErrorBoundary from 'plugins/pluggable/error_boundary';

import {blockMattermostLinks, isMattermostURL} from './mattermost_links';

describe('mattermost links', () => {
    let open: jest.Mock;
    let undo: () => void;
    beforeEach(() => {
        open = jest.fn(() => null);
        window.open = open;
        undo = blockMattermostLinks();
    });
    afterEach(() => {
        undo();
        document.body.innerHTML = '';
    });

    const click = (el: Element) => {
        const event = new MouseEvent('click', {bubbles: true, cancelable: true});
        el.dispatchEvent(event);
        return event;
    };

    // A button whose click opens a window, as app and Antimatter code do.
    const opener = (parent: Element, url: string) => {
        const button = document.createElement('button');
        button.addEventListener('click', () => window.open(url, '_blank'));
        parent.appendChild(button);
        return button;
    };

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

    test('Antimatter\'s own UI doesn\'t open them, nor follow their links', () => {
        window.open('https://example.com', '_blank');
        expect(open).toHaveBeenCalledWith('https://example.com', '_blank');

        const link = document.createElement('a');
        link.href = 'https://mattermost.com/download';
        document.body.appendChild(link);
        expect(click(link).defaultPrevented).toBe(true);

        open.mockClear();
        click(opener(document.body, 'https://docs.mattermost.com'));
        expect(open).not.toHaveBeenCalled();

        undo();
        expect(window.open).toBe(open);
        undo = blockMattermostLinks();
    });

    // compat-plugins #6: the block dropped every link to *.mattermost.com, those in messages and apps' UI included.
    test('a link to mattermost.com in a message stays clickable', () => {
        const message = document.createElement('div');
        message.className = 'am-msg';
        message.innerHTML = '<div class="post-message__text"><p><a href="https://docs.mattermost.com/guide">docs</a></p></div>';
        document.body.appendChild(message);
        expect(click(message.querySelector('a')!).defaultPrevented).toBe(false);

        click(opener(message, 'https://docs.mattermost.com/guide'));
        expect(open).toHaveBeenCalledWith('https://docs.mattermost.com/guide', '_blank');
    });

    test('an app\'s links and windows to mattermost.com work', () => {
        // Apps' components are drawn inside one, as Pluggable does.
        const {unmount} = render(
            React.createElement(PluggableErrorBoundary, {pluginId: 'calls'} as React.ComponentProps<typeof PluggableErrorBoundary>,
                React.createElement('a', {href: 'https://mattermost.com/pl/calls', id: 'app-link'}, 'Learn more'),
                React.createElement('button', {id: 'app-button', onClick: () => window.open('https://docs.mattermost.com/calls')}, 'Help'),
            ),
        );

        expect(click(document.getElementById('app-link')!).defaultPrevented).toBe(false);
        act(() => {
            (document.getElementById('app-button') as HTMLButtonElement).click();
        });
        expect(open).toHaveBeenCalledWith('https://docs.mattermost.com/calls');
        unmount();
    });

    test('a window opened outside any event (after an app\'s request) is opened', () => {
        window.open('https://docs.mattermost.com/guide');
        expect(open).toHaveBeenCalledWith('https://docs.mattermost.com/guide');
    });
});
