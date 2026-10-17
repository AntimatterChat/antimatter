// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Post} from '@mattermost/types/posts';

import {renderWithContext, screen, userEvent, waitFor, within} from 'tests/react_testing_utils';

import {clearMessageActions} from './message_actions';
import PluginMenuActions from './plugin_menu_actions';

const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));

const action = (id: string, text: React.ReactNode, extra: Record<string, unknown> = {}) => ({id, pluginId: 'p', text, action: jest.fn(), ...extra});

const post = {id: 'post1', channel_id: 'chan1', type: '', message: 'hello', pending_post_id: ''} as unknown as Post;

describe('fusion/messages/PluginMenuActions', () => {
    const realFetch = global.fetch;
    let fetchMock: jest.Mock;

    beforeEach(() => {
        clearMessageActions();
        mockToast.mockClear();
        fetchMock = jest.fn(() => Promise.resolve({ok: true, status: 200, json: () => Promise.resolve([])}));
        global.fetch = fetchMock as unknown as typeof fetch;
    });
    afterEach(() => {
        global.fetch = realFetch;
    });

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

    describe('integrations', () => {
        const withWebhooks = {entities: {general: {config: {EnableOutgoingWebhooks: 'true'}}}};
        const archive = {id: 'hook1', team_id: 't1', channel_id: 'chan1', display_name: 'Archive', description: 'Files the message', icon_url: '/api/v4/hooks/outgoing/hook1/icon?_=1'};
        const crm = {id: 'hook2', team_id: 't1', channel_id: '', display_name: 'CRM', description: '', icon_url: ''};
        const listed = (actions: unknown[], answer: unknown = {}) => fetchMock.mockImplementation((url: string) => Promise.resolve({
            ok: true,
            status: 200,
            json: () => Promise.resolve(url.endsWith('/message_actions') ? actions : answer),
        }));
        const sendCall = () => fetchMock.mock.calls.find(([url]) => (url as string).includes('/posts/'));

        test('lists the channel\'s message actions under their own heading, after the apps\' actions', async () => {
            listed([archive, crm]);
            render([action('a1', 'Create a task')], withWebhooks);

            const menu = screen.getByRole('menu', {name: 'More message actions'});
            await waitFor(() => expect(within(menu).getByText('Integrations')).toBeInTheDocument());
            expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual(['Create a task', 'Archive', 'CRM']);
            expect(fetchMock.mock.calls[0][0]).toMatch(/\/api\/v4\/channels\/chan1\/message_actions$/);
        });

        test('shows More actions for integrations alone', async () => {
            listed([crm]);
            render([], withWebhooks);
            expect(await screen.findByRole('menuitem', {name: 'More actions'})).toBeInTheDocument();
        });

        test('an integration shows its uploaded icon, or the neutral icon without one', async () => {
            listed([archive, crm]);
            render([], withWebhooks);
            const archiveItem = await screen.findByRole('menuitem', {name: 'Archive'});

            expect(archiveItem.querySelector('.am-mi-app img')?.getAttribute('src')).toMatch(/\/api\/v4\/hooks\/outgoing\/hook1\/icon\?_=1$/);
            expect(archiveItem.querySelector('.am-mi-app')).toHaveAttribute('aria-hidden', 'true');
            expect(archiveItem).toHaveAttribute('title', 'Files the message');
            expect(screen.getByRole('menuitem', {name: 'CRM'}).querySelector('.am-mi-app use')).toHaveAttribute('href', '#am-i-plug');
        });

        test('an icon URL goes through the image proxy when there is one', async () => {
            listed([{...crm, icon_url: 'https://crm.example/logo.png'}]);
            render([], {entities: {general: {config: {EnableOutgoingWebhooks: 'true', HasImageProxy: 'true'}}}});
            const item = await screen.findByRole('menuitem', {name: 'CRM'});
            expect(item.querySelector('img')?.getAttribute('src')).toContain('/api/v4/image?url=' + encodeURIComponent('https://crm.example/logo.png'));
        });

        test('picking one sends the message, closes the menu and tells it was sent', async () => {
            listed([archive]);
            const onDone = render([], withWebhooks);
            await userEvent.click(await screen.findByRole('menuitem', {name: 'Archive'}));

            expect(onDone).toHaveBeenCalled();
            const [url, options] = sendCall()!;
            expect(url).toMatch(/\/api\/v4\/posts\/post1\/message_actions\/hook1$/);
            expect((options as RequestInit).method).toBe('post');
            await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Sent to Archive'));
        });

        test('the integration\'s ephemeral text replaces the notice', async () => {
            listed([archive], {ephemeral_text: 'Filed as ticket 42'});
            render([], withWebhooks);
            await userEvent.click(await screen.findByRole('menuitem', {name: 'Archive'}));
            await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Filed as ticket 42'));
        });

        test('a refusal shows the server\'s reason', async () => {
            fetchMock.mockImplementation((url: string) => Promise.resolve(url.endsWith('/message_actions') ? {ok: true, status: 200, json: () => Promise.resolve([archive])} : {ok: false, status: 429, json: () => Promise.resolve({message: 'Too many'})}));
            render([], withWebhooks);
            await userEvent.click(await screen.findByRole('menuitem', {name: 'Archive'}));
            await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Too many'));
        });

        test('nothing is asked when outgoing webhooks are off', () => {
            render([action('a1', 'Create a task')]);
            expect(fetchMock).not.toHaveBeenCalled();
            expect(screen.queryByText('Integrations')).not.toBeInTheDocument();
        });

        test('nor for a system message', () => {
            render([], withWebhooks, {...post, type: 'system_join_channel'} as Post);
            expect(fetchMock).not.toHaveBeenCalled();
        });
    });
});
