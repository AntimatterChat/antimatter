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
                    label='More actions'
                    menuLabel='More message actions'
                    run={run}
                />
            </div>,
            {plugins: {components: {PostDropdownMenu: actions}}} as never,
        );
        return onDone;
    };

    test('puts the single actions under the More actions flyout, and runs them on the message', async () => {
        const remind = action('a1', 'Remind me');
        const onDone = render([remind, action('a2', 'Create a task')]);
        expect(screen.getByRole('menuitem', {name: 'More actions'})).toBeInTheDocument();
        const menu = screen.getByRole('menu', {name: 'More message actions'});
        expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual(['Remind me', 'Create a task']);

        await userEvent.click(within(menu).getByRole('menuitem', {name: 'Remind me'}));
        expect(remind.action).toHaveBeenCalledWith('post1');
        expect(onDone).toHaveBeenCalled();
    });

    test('the AI actions are a group inside More actions, not an entry of their own', async () => {
        const summarize = action('s1', 'Summarize');
        render([
            action('a1', 'Create a task'),
            action('ai', 'AI actions', {pluginId: 'mattermost-ai', subMenu: [summarize, action('s2', 'Translate')]}),
        ]);

        // One flyout only: no "AI actions" entry in the message menu itself.
        expect(screen.queryByRole('menuitem', {name: 'AI actions'})).not.toBeInTheDocument();
        expect(screen.queryByRole('menu', {name: 'AI actions'})).not.toBeInTheDocument();
        const menu = screen.getByRole('menu', {name: 'More message actions'});
        expect(within(menu).getByText('AI actions')).toBeInTheDocument();
        expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual(['Create a task', 'Summarize', 'Translate']);

        await userEvent.click(within(menu).getByRole('menuitem', {name: 'Summarize'}));
        expect(summarize.action).toHaveBeenCalledWith('post1');
    });

    test('shows More actions for a submenu alone', () => {
        render([action('ai', 'AI actions', {pluginId: 'mattermost-ai', subMenu: [action('s1', 'Summarize')]})]);
        expect(screen.getByRole('menuitem', {name: 'More actions'})).toBeInTheDocument();
        expect(within(screen.getByRole('menu', {name: 'More message actions'})).getAllByRole('menuitem').map((i) => i.textContent)).toEqual(['Summarize']);
    });

    test('leaves out actions whose filter refuses the message, and empty submenus', () => {
        render([
            action('a1', 'Hidden', {filter: (postId: string) => postId !== 'post1'}),
            action('g', 'Group', {subMenu: [action('s', 'Never', {filter: () => false})]}),
        ]);
        expect(screen.queryAllByRole('menuitem')).toHaveLength(0);
        expect(screen.queryByText('Group')).not.toBeInTheDocument();
    });
});
