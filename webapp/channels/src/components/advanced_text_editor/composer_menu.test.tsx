// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import type {GlobalState} from 'types/store';
import type {ComposerMenuItemRegistration} from 'types/store/plugins';

import ComposerMenu from './composer_menu';

function makeItem(partial: Partial<ComposerMenuItemRegistration> = {}): ComposerMenuItemRegistration {
    return {
        id: 'poll',
        pluginId: 'polls',
        text: 'Create a poll',
        icon: <i className='icon-poll'/>,
        action: jest.fn(),
        shouldRender: () => true,
        ...partial,
    };
}

function stateWith(items: ComposerMenuItemRegistration[]): DeepPartial<GlobalState> {
    return {
        plugins: {
            components: {
                ComposerMenuItem: items,
            },
        },
    };
}

describe('components/advanced_text_editor/ComposerMenu', () => {
    test('renders nothing without plugin items', () => {
        renderWithContext(
            <ComposerMenu
                channelId='channel'
                rootId=''
            />,
            stateWith([]),
        );

        expect(screen.queryByRole('button', {name: 'Add to message'})).not.toBeInTheDocument();
    });

    test('renders nothing when no item renders in this message box', () => {
        renderWithContext(
            <ComposerMenu
                channelId='channel'
                rootId=''
            />,
            stateWith([makeItem({shouldRender: (_state, ctx) => Boolean(ctx.rootId)})]),
        );

        expect(screen.queryByRole('button', {name: 'Add to message'})).not.toBeInTheDocument();
    });

    test('lists the items and runs the clicked one with the channel and thread', async () => {
        const action = jest.fn();
        renderWithContext(
            <ComposerMenu
                channelId='channel'
                rootId='root'
            />,
            stateWith([
                makeItem({action}),
                makeItem({id: 'hidden', text: 'Hidden item', shouldRender: () => false}),
            ]),
        );

        await userEvent.click(screen.getByRole('button', {name: 'Add to message'}));

        expect(screen.queryByText('Hidden item')).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('menuitem', {name: 'Create a poll'}));

        // Menu items run after the menu's close animation.
        await waitFor(() => expect(action).toHaveBeenCalledWith({channelId: 'channel', rootId: 'root'}));
    });

    test('passes no rootId outside threads', async () => {
        const action = jest.fn();
        renderWithContext(
            <ComposerMenu
                channelId='channel'
                rootId=''
            />,
            stateWith([makeItem({action})]),
        );

        await userEvent.click(screen.getByRole('button', {name: 'Add to message'}));
        await userEvent.click(screen.getByRole('menuitem', {name: 'Create a poll'}));

        await waitFor(() => expect(action).toHaveBeenCalledWith({channelId: 'channel', rootId: undefined}));
    });
});
