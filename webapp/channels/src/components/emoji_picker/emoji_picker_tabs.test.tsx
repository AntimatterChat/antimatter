// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import type {GlobalState} from 'types/store';
import type {EmojiPickerTabProps, EmojiPickerTabRegistration} from 'types/store/plugins';

import EmojiPickerTabs from './emoji_picker_tabs';

jest.mock('components/emoji_picker', () => () => <div>{'emoji picker'}</div>);
jest.mock('components/async_load', () => ({
    makeAsyncComponent: () => () => <div>{'giphy picker'}</div>,
}));

function makeTab(partial: Partial<EmojiPickerTabRegistration> = {}): EmojiPickerTabRegistration {
    return {
        id: 'stickers',
        pluginId: 'gifs',
        label: 'Stickers',
        order: 0,
        component: ({channelId, rootId, filter}: EmojiPickerTabProps) => <div>{`stickers of ${channelId}/${rootId}, search "${filter}"`}</div>,
        shouldRender: () => true,
        replacesGifPicker: false,
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

const baseProps = {
    onEmojiClose: jest.fn(),
    onEmojiClick: jest.fn(),
    onGifClick: jest.fn(),
    enableGifPicker: true,
    channelId: 'channel',
    rootId: 'root',
    showPluginTabs: true,
};

describe('components/emoji_picker/EmojiPickerTabs', () => {
    test('shows the Emojis and GIFs tabs without plugin tabs', () => {
        renderWithContext(<EmojiPickerTabs {...baseProps}/>, stateWith([]));

        expect(screen.getByRole('tab', {name: 'Emojis'})).toBeInTheDocument();
        expect(screen.getByRole('tab', {name: 'GIFs'})).toBeInTheDocument();
        expect(screen.getAllByRole('tab')).toHaveLength(2);
    });

    test('shows the plugin tabs after the core ones, with the message box and search', async () => {
        renderWithContext(<EmojiPickerTabs {...baseProps}/>, stateWith([makeTab()]));

        expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(['Emojis', 'GIFs', 'Stickers']);

        await userEvent.click(screen.getByRole('tab', {name: 'Stickers'}));
        expect(await screen.findByText('stickers of channel/root, search ""')).toBeInTheDocument();
    });

    test('closes the picker when the plugin tab is done', async () => {
        const onEmojiClose = jest.fn();
        const component = ({onSelectDone, isFusion}: EmojiPickerTabProps) => (
            <button onClick={onSelectDone}>{isFusion ? 'fusion' : 'classic'}</button>
        );
        renderWithContext(
            <EmojiPickerTabs
                {...baseProps}
                onEmojiClose={onEmojiClose}
            />,
            stateWith([makeTab({component})]),
        );

        await userEvent.click(screen.getByRole('tab', {name: 'Stickers'}));
        await userEvent.click(await screen.findByRole('button', {name: 'classic'}));
        expect(onEmojiClose).toHaveBeenCalled();
    });

    test('shows the plugin tabs even when the GIF picker is off', () => {
        renderWithContext(
            <EmojiPickerTabs
                {...baseProps}
                enableGifPicker={false}
            />,
            stateWith([makeTab()]),
        );

        expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(['Emojis', 'Stickers']);
    });

    test('hides the plugin tabs when not composing', () => {
        renderWithContext(
            <EmojiPickerTabs
                {...baseProps}
                showPluginTabs={false}
            />,
            stateWith([makeTab()]),
        );

        expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(['Emojis', 'GIFs']);
    });

    test('hides the plugin tabs whose shouldRender is false', () => {
        renderWithContext(<EmojiPickerTabs {...baseProps}/>, stateWith([makeTab({shouldRender: () => false})]));

        expect(screen.getAllByRole('tab').map((t) => t.textContent)).toEqual(['Emojis', 'GIFs']);
    });
});
