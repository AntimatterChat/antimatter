// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {DeepPartial} from '@mattermost/types/utilities';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import type {GlobalState} from 'types/store';
import type {ComposerMenuItemRegistration} from 'types/store/plugins';

import PluginMenuItems from './plugin_menu_items';

function makeItem(partial: Partial<ComposerMenuItemRegistration> = {}): ComposerMenuItemRegistration {
    return {
        id: 'poll',
        pluginId: 'polls',
        text: 'Create a poll',
        icon: <i data-testid='plugin-icon'/>,
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

describe('fusion/composer/PluginMenuItems', () => {
    test('uses the Fusion icon a plugin names, else its own icon', () => {
        const {container} = renderWithContext(
            <PluginMenuItems
                channelId='channel'
                rootId=''
                onClose={jest.fn()}
            />,
            stateWith([
                makeItem({fusionIcon: 'poll'}),
                makeItem({id: 'unknown', text: 'Unknown icon', fusionIcon: 'no-such-icon'}),
            ]),
        );

        expect(container.querySelector('use')?.getAttribute('href')).toBe('#am-i-poll');
        expect(screen.getAllByTestId('plugin-icon')).toHaveLength(1);
        expect(screen.getByRole('menuitem', {name: 'Unknown icon'})).toContainElement(screen.getByTestId('plugin-icon'));
    });

    test('closes the menu and runs the item with the channel and thread', async () => {
        const action = jest.fn();
        const onClose = jest.fn();
        renderWithContext(
            <PluginMenuItems
                channelId='channel'
                rootId='root'
                onClose={onClose}
            />,
            stateWith([
                makeItem({action}),
                makeItem({id: 'hidden', text: 'Hidden item', shouldRender: (_state, ctx) => !ctx.rootId}),
            ]),
        );

        expect(screen.queryByText('Hidden item')).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('menuitem', {name: 'Create a poll'}));

        expect(onClose).toHaveBeenCalled();
        expect(action).toHaveBeenCalledWith({channelId: 'channel', rootId: 'root'});
    });
});
