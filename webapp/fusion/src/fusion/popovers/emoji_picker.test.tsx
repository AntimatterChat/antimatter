// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import type {GlobalState} from 'types/store';
import type {EmojiPickerTabProps, EmojiPickerTabRegistration} from 'types/store/plugins';

import EmojiPicker from './emoji_picker';

// Permissions are the gate's business: here, the user may create emoji.
jest.mock('components/permissions_gates/any_team_permission_gate', () => ({children}: {children: React.ReactNode}) => children);

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

    test('puts the plugin tabs of order 10 or more after Emoji', () => {
        renderWithContext(
            <EmojiPicker
                anchor={null}
                onPick={jest.fn()}
                onClose={jest.fn()}
                compose={{channelId: 'channel', insertText: jest.fn()}}
            />,
            stateWith([makeTab({id: 'memes', label: 'Memes', order: 20}), ...tabs]),
        );

        expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(['GIFs', 'Stickers', 'Emoji', 'Memes']);
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

    test('opens the custom emoji dialog when custom emoji are enabled', async () => {
        const onClose = jest.fn();
        const state = {
            entities: {
                general: {config: {EnableCustomEmoji: 'true'}},
                teams: {currentTeamId: 'team1', teams: {team1: {id: 'team1', name: 'lab'}}},
            },
        } as DeepPartial<GlobalState>;
        renderWithContext(
            <EmojiPicker
                anchor={null}
                onPick={jest.fn()}
                onClose={onClose}
            />,
            state,
        );
        await userEvent.click(screen.getByRole('button', {name: 'Add custom emoji'}));
        expect(onClose).toHaveBeenCalled();
    });

    test('has no custom emoji button when custom emoji are off', () => {
        renderWithContext(
            <EmojiPicker
                anchor={null}
                onPick={jest.fn()}
                onClose={jest.fn()}
            />,
            {entities: {teams: {currentTeamId: 'team1', teams: {team1: {id: 'team1', name: 'lab'}}}}} as DeepPartial<GlobalState>,
        );
        expect(screen.queryByRole('button', {name: 'Add custom emoji'})).not.toBeInTheDocument();
    });

    // The search ranks recently used emoji first: it must get their ids, not their names (an unknown id crashed the
    // whole app on the first letter typed).
    test('searches emoji, recently used ones first', async () => {
        // The grids draw at once here.
        const lazy = window.IntersectionObserver;
        window.IntersectionObserver = class {
            constructor(private callback: IntersectionObserverCallback) {}
            observe() {
                this.callback([{isIntersecting: true} as IntersectionObserverEntry], this as unknown as IntersectionObserver);
            }
            disconnect() {}
        } as unknown as typeof IntersectionObserver;
        const onPick = jest.fn();
        renderWithContext(
            <EmojiPicker
                anchor={null}
                onPick={onPick}
                onClose={jest.fn()}
            />,
            {} as DeepPartial<GlobalState>,
        );

        await userEvent.type(screen.getByRole('textbox', {name: 'Search emoji'}), 'ta');

        expect(screen.getByRole('heading', {name: 'Search results'})).toBeInTheDocument();
        const results = screen.getAllByRole('button', {name: /ta/});

        // :tada: is one of the usual reactions shown as recently used until anything is.
        expect(results[0]).toHaveAccessibleName('tada');
        await userEvent.click(results[0]);
        expect(onPick).toHaveBeenCalledWith('tada');

        window.IntersectionObserver = lazy;
    });
});
