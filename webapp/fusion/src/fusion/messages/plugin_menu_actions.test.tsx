// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Post} from '@mattermost/types/posts';

import {renderWithContext, screen, userEvent, within} from 'tests/react_testing_utils';

import PluginMenuActions from './plugin_menu_actions';

const action = (id: string, text: React.ReactNode, extra: Record<string, unknown> = {}) => ({id, pluginId: 'p', text, action: jest.fn(), ...extra});

const post = {id: 'post1', channel_id: 'chan1', type: '', message: 'hello', pending_post_id: ''} as unknown as Post;

describe('fusion/messages/PluginMenuActions', () => {
    const render = (actions: Array<ReturnType<typeof action>>, extraState: Record<string, unknown> = {}, forPost: Post = post) => {
        const onDone = jest.fn();
        const run = (fn: () => void) => () => {
            fn();
            onDone();
        };
        const {plugins, ...rest} = extraState as {plugins?: {components?: Record<string, unknown>}};
        renderWithContext(
            <div role='menu'>
                <PluginMenuActions
                    post={forPost}
                    label='More actions'
                    menuLabel='More message actions'
                    run={run}
                />
            </div>,
            {plugins: {components: {PostDropdownMenu: actions, ...plugins?.components}}, ...rest} as never,
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

    describe('app icons', () => {
        const iconOf = (name: string) => screen.getByRole('menuitem', {name}).querySelector('.am-mi-app');

        test('an app the mockup knows shows its Fusion icon, hidden from screen readers', () => {
            render([action('pb', 'Run playbook', {pluginId: 'playbooks'})]);
            const icon = iconOf('Run playbook');
            expect(icon).toHaveAttribute('aria-hidden', 'true');
            expect(icon?.querySelector('use')).toHaveAttribute('href', '#am-i-runbook');
        });

        test('another app shows the icon of its app bar entry', () => {
            render([action('x', 'Send to Acme', {pluginId: 'com.acme'})], {
                plugins: {components: {AppBar: [{id: 'bar', pluginId: 'com.acme', iconUrl: 'https://acme.example/icon.png'}]}},
            });
            expect(iconOf('Send to Acme')?.querySelector('img')).toHaveAttribute('src', 'https://acme.example/icon.png');
        });

        test('an app without an icon gets the neutral app icon', () => {
            render([action('x', 'Do the thing', {pluginId: 'com.noicon'})]);
            expect(iconOf('Do the thing')?.querySelector('use')).toHaveAttribute('href', '#am-i-apps');
        });

        test('the icon an app draws in its own text is hidden, the app icon standing for it', () => {
            const text = (
                <>
                    <span className='MenuItem__icon'><i className='CompassIcon icon-product-playbooks'/></span>
                    {'Run playbook'}
                </>
            );
            render([action('pb', text, {pluginId: 'playbooks'})]);
            const item = screen.getByRole('menuitem', {name: 'Run playbook'});
            expect(item.querySelector('.MenuItem__icon')).toHaveClass('am-mi-own-ic');
            expect(item.querySelectorAll('.am-mi-app')).toHaveLength(1);
        });

        test('a text that starts with words keeps them', () => {
            render([action('x', <span>{'Create a task'}</span>, {pluginId: 'com.tasks'})]);
            expect(screen.getByRole('menuitem', {name: 'Create a task'}).querySelector('.am-mi-own-ic')).toBeNull();
        });
    });
});
