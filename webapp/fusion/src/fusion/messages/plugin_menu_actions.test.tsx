// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent, within} from 'tests/react_testing_utils';

import PluginMenuActions from './plugin_menu_actions';

const action = (id: string, text: string, extra: Record<string, unknown> = {}) => ({id, pluginId: 'p', text, action: jest.fn(), ...extra});

describe('fusion/messages/PluginMenuActions', () => {
    const render = (actions: Array<ReturnType<typeof action>>) => {
        const onDone = jest.fn();
        const run = (fn: () => void) => () => {
            fn();
            onDone();
        };
        renderWithContext(
            <div role='menu'>
                <PluginMenuActions
                    postId='post1'
                    label='Apps'
                    menuLabel='App actions'
                    run={run}
                />
            </div>,
            {plugins: {components: {PostDropdownMenu: actions}}} as never,
        );
        return onDone;
    };

    test('puts the single actions under one flyout, and runs them on the message', async () => {
        const remind = action('a1', 'Remind me');
        const onDone = render([remind, action('a2', 'Create a task')]);
        expect(screen.getByRole('menuitem', {name: 'Apps'})).toBeInTheDocument();
        const menu = screen.getByRole('menu', {name: 'App actions'});
        expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual(['Remind me', 'Create a task']);

        await userEvent.click(within(menu).getByRole('menuitem', {name: 'Remind me'}));
        expect(remind.action).toHaveBeenCalledWith('post1');
        expect(onDone).toHaveBeenCalled();
    });

    test('gives each submenu its own flyout, the known app\'s icon on it', () => {
        const summarize = action('s1', 'Summarize');
        render([action('ai', 'AI actions', {pluginId: 'mattermost-ai', subMenu: [summarize, action('s2', 'Translate')]})]);
        const menu = screen.getByRole('menu', {name: 'AI actions'});
        expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual(['Summarize', 'Translate']);
        expect(screen.queryByRole('menuitem', {name: 'Apps'})).not.toBeInTheDocument();
    });

    test('leaves out actions whose filter refuses the message, and empty submenus', () => {
        render([
            action('a1', 'Hidden', {filter: (postId: string) => postId !== 'post1'}),
            action('g', 'Group', {subMenu: [action('s', 'Never', {filter: () => false})]}),
        ]);
        expect(screen.queryAllByRole('menuitem')).toHaveLength(0);
    });
});
