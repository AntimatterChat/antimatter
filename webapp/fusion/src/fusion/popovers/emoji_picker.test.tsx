// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import type {GlobalState} from 'types/store';
import type {EmojiPickerTabProps, EmojiPickerTabRegistration} from 'types/store/plugins';

import EmojiPicker from './emoji_picker';

function makeTab(partial: Partial<EmojiPickerTabRegistration> = {}): EmojiPickerTabRegistration {
    return {
        id: 'gifs',
        pluginId: 'gifs-plugin',
        label: 'GIFs',
        order: 0,
        component: ({channelId, rootId, isFusion, onSelectDone, insertText}: EmojiPickerTabProps) => (
            <button
                onClick={() => {
                    insertText?.('gif');
                    onSelectDone();
                }}
            >
                {`gifs of ${channelId}/${rootId}${isFusion ? ' in Fusion' : ''}`}
            </button>
        ),
        shouldRender: () => true,
        replacesGifPicker: true,
        ...partial,
    };
}

function stateWith(tabs: EmojiPickerTabRegistration[]): DeepPartial<GlobalState> {
    return {
        plugins: {
            components: {
                EmojiPickerTab: tabs,
            },
        },
    };
}

const tabs = [makeTab(), makeTab({id: 'stickers', label: 'Stickers', order: 1, replacesGifPicker: false})];

describe('fusion/popovers/EmojiPicker', () => {
    beforeAll(() => {
        // The emoji grids draw once they scroll into view.
        window.IntersectionObserver = class {
            observe() {}
            disconnect() {}
        } as unknown as typeof IntersectionObserver;
    });

    beforeEach(() => {
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);
    });

    afterEach(() => {
        document.getElementById(LAYER_ID)?.remove();
    });

    test('has no tabs for reactions', () => {
        renderWithContext(
            <EmojiPicker
                anchor={null}
                onPick={jest.fn()}
                onClose={jest.fn()}
            />,
            stateWith(tabs),
        );

        expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
        expect(screen.getByRole('textbox', {name: 'Search emoji'})).toBeInTheDocument();
    });

    test('has no tabs in the composer without plugin tabs', () => {
        renderWithContext(
            <EmojiPicker
                anchor={null}
                onPick={jest.fn()}
                onClose={jest.fn()}
                compose={{channelId: 'channel', insertText: jest.fn()}}
            />,
            stateWith([]),
        );

        expect(screen.queryByRole('tablist')).not.toBeInTheDocument();
    });

    test('shows the plugin tabs before Emoji in the composer and opens on the last tab used', async () => {
        const onClose = jest.fn();
        const insertText = jest.fn();
        const picker = (
            <EmojiPicker
                anchor={null}
                onPick={jest.fn()}
                onClose={onClose}
                keepOpen={true}
                compose={{channelId: 'channel', rootId: 'root', insertText}}
            />
        );
        const {unmount} = renderWithContext(picker, stateWith(tabs));

        expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(['GIFs', 'Stickers', 'Emoji']);
        expect(screen.getByRole('tab', {name: 'Emoji'})).toHaveAttribute('aria-selected', 'true');

        await userEvent.click(screen.getByRole('tab', {name: 'GIFs'}));
        expect(screen.getByRole('tab', {name: 'GIFs'})).toHaveAttribute('aria-selected', 'true');
        expect(screen.queryByRole('textbox', {name: 'Search emoji'})).not.toBeInTheDocument();

        await userEvent.click(screen.getByRole('button', {name: 'gifs of channel/root in Fusion'}));
        expect(insertText).toHaveBeenCalledWith('gif');
        expect(onClose).toHaveBeenCalled();

        unmount();
        renderWithContext(picker, stateWith(tabs));
        expect(screen.getByRole('tab', {name: 'GIFs'})).toHaveAttribute('aria-selected', 'true');
    });

    test('falls back to the Emoji tab when the last tab is gone', () => {
        renderWithContext(
            <EmojiPicker
                anchor={null}
                onPick={jest.fn()}
                onClose={jest.fn()}
                compose={{channelId: 'channel', insertText: jest.fn()}}
            />,
            stateWith([tabs[1]]),
        );

        expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(['Stickers', 'Emoji']);
        expect(screen.getByRole('tab', {name: 'Emoji'})).toHaveAttribute('aria-selected', 'true');
        expect(screen.getByRole('textbox', {name: 'Search emoji'})).toBeInTheDocument();
    });
});
