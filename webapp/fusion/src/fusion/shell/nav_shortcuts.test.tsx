// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {fireEvent, renderWithContext} from 'tests/react_testing_utils';

import {LayoutProvider} from './layout_context';
import {next, useNavShortcuts} from './nav_shortcuts';

describe('next', () => {
    test('goes round the list both ways', () => {
        expect(next(['a', 'b', 'c'], 'b', 1)).toBe('c');
        expect(next(['a', 'b', 'c'], 'c', 1)).toBe('a');
        expect(next(['a', 'b', 'c'], 'a', -1)).toBe('c');
    });

    test('starts at an end when the current one is not in the list', () => {
        expect(next(['a', 'b'], 'x', 1)).toBe('a');
        expect(next(['a', 'b'], 'x', -1)).toBe('b');
        expect(next([], 'x', 1)).toBeUndefined();
    });
});

function Harness() {
    useNavShortcuts();
    return null;
}

// Builds the composer's DOM the way composer.tsx renders it, with either the plain-text textarea or the rich text
// editor's ProseMirror contenteditable (never both), and returns the element the shortcut should focus.
function buildComposer(rich: boolean): HTMLElement {
    const main = document.createElement('div');
    main.className = 'am-main';
    const composer = document.createElement('div');
    composer.className = 'am-composer';
    main.appendChild(composer);

    let target: HTMLElement;
    if (rich) {
        const richInput = document.createElement('div');
        richInput.className = 'am-rich-input';
        const prose = document.createElement('div');
        prose.className = 'ProseMirror';
        prose.contentEditable = 'true';
        prose.tabIndex = 0;
        richInput.appendChild(prose);
        composer.appendChild(richInput);
        target = prose;
    } else {
        target = document.createElement('textarea');
        composer.appendChild(target);
    }

    document.body.appendChild(main);
    return target;
}

describe('useNavShortcuts', () => {
    const render = () => renderWithContext(
        <LayoutProvider><Harness/></LayoutProvider>,
        {
            entities: {
                users: {currentUserId: 'me'},
                preferences: {myPreferences: {}},
            },
        } as never,
    );

    const pressCtrlShiftL = () => {
        fireEvent.keyDown(document, {key: 'L', code: 'KeyL', ctrlKey: true, shiftKey: true});
    };

    afterEach(() => {
        document.querySelectorAll('.am-main').forEach((el) => el.remove());
    });

    test('focuses the plain-text textarea when the rich text editor is off', () => {
        render();
        const textarea = buildComposer(false);

        pressCtrlShiftL();

        expect(document.activeElement).toBe(textarea);
    });

    test('focuses the rich text editor when it is on', () => {
        render();
        const rich = buildComposer(true);

        pressCtrlShiftL();

        expect(document.activeElement).toBe(rich);
    });
});
