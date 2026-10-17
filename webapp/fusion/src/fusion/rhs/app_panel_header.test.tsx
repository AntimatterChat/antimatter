// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import fs from 'fs';
import path from 'path';

import React, {useRef} from 'react';

import {fireEvent, renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import AppPanelHeader from './app_panel_header';
import {PanelResizeHandle, usePanelWidth} from './panel_width';

// jsdom has no pointer events: mouse events with a pointer id stand in for them.
if (!window.PointerEvent) {
    class PointerEvent extends MouseEvent {
        pointerId: number;
        constructor(type: string, init: PointerEventInit = {}) {
            super(type, init);
            this.pointerId = init.pointerId ?? 0;
        }
    }
    window.PointerEvent = PointerEvent as typeof window.PointerEvent;
}

// Apps of every kind: the default apps (Kanban's id is focalboard), one the mockup doesn't know, one with an icon in
// the app rail, one with nothing at all.
const APP_IDS = ['playbooks', 'focalboard', 'com.antimatterchat.notes', 'com.mattermost.plugin-todo', 'com.example.anything', 'with.rail.icon', 'bare'];

function stateFor(pluginId: string) {
    return {
        plugins: {
            plugins: {[pluginId]: {id: pluginId, name: `Manifest ${pluginId}`}},
            components: {
                AppBar: pluginId === 'with.rail.icon' ? [{id: 'bar', pluginId, iconUrl: '/icon.svg', tooltipText: 'Rail label'}] : [],
                ChannelHeaderButton: [],
                RightHandSidebarComponent: [],
            },
        },
    } as never;
}

// The header's parts, in their order: what each child is.
function parts() {
    return Array.from(screen.getByTestId('app-panel-header').children).map((el) => {
        if (el.classList.contains('am-rhs-app-ic')) {
            return 'icon';
        }
        if (el.tagName === 'H3') {
            return 'name';
        }
        return el.getAttribute('aria-label');
    });
}

describe('fusion/rhs/AppPanelHeader', () => {
    test.each(APP_IDS)('is exactly icon, name, restore size, new window and close, in that order, for %s', (pluginId) => {
        renderWithContext(
            <AppPanelHeader
                pluginId={pluginId}
                sized={true}
                onRestoreSize={jest.fn()}
                onNewWindow={jest.fn()}
                onClose={jest.fn()}
            />,
            stateFor(pluginId),
        );

        expect(parts()).toEqual(['icon', 'name', "Restore the panel's size", 'Open in a new window', 'Close']);
    });

    test.each(APP_IDS)('has no restore size button for %s at its default size', (pluginId) => {
        renderWithContext(
            <AppPanelHeader
                pluginId={pluginId}
                sized={false}
                onRestoreSize={jest.fn()}
                onNewWindow={jest.fn()}
                onClose={jest.fn()}
            />,
            stateFor(pluginId),
        );

        expect(parts()).toEqual(['icon', 'name', 'Open in a new window', 'Close']);
    });

    test('names the app: its title when text, else its rail label, else its manifest name, else its id', () => {
        const nameOf = (pluginId: string, title?: React.ReactNode, state = stateFor(pluginId)) => {
            const {unmount} = renderWithContext(
                <AppPanelHeader
                    pluginId={pluginId}
                    title={title}
                />,
                state,
            );
            const name = screen.getByRole('heading').textContent;
            unmount();
            return name;
        };

        expect(nameOf('with.rail.icon', 'Registered title')).toBe('Registered title');
        expect(nameOf('with.rail.icon')).toBe('Rail label');
        expect(nameOf('with.rail.icon', <b>{'Not text'}</b>)).toBe('Rail label');
        expect(nameOf('com.example.anything')).toBe('Manifest com.example.anything');
        expect(nameOf('unknown.app', undefined, stateFor('other'))).toBe('unknown.app');
    });

    test("has no slot or portal for an app's controls: a title that isn't text never reaches the header", () => {
        renderWithContext(
            <AppPanelHeader
                pluginId='com.example.anything'
                title={
                    <span className='app-own-header'>
                        <button type='button'>{'All boards'}</button>
                        <span className='app-head-slot'/>
                    </span>
                }
                onClose={jest.fn()}
            />,
            stateFor('com.example.anything'),
        );

        const header = screen.getByTestId('app-panel-header');
        expect(screen.queryByRole('button', {name: 'All boards'})).toBeNull();
        expect(header.querySelector('.app-own-header, .app-head-slot')).toBeNull();
        expect(header).toHaveTextContent('Manifest com.example.anything');

        // Nothing in it but the icon, the name and the header's own buttons: no empty element left for an app to
        // fill.
        expect(parts()).toEqual(['icon', 'name', 'Close']);
        expect(Array.from(header.children).filter((el) => !el.childNodes.length)).toHaveLength(0);
    });

    test('draws the icon of the app in the app rail', () => {
        renderWithContext(<AppPanelHeader pluginId='with.rail.icon'/>, stateFor('with.rail.icon'));
        expect(screen.getByTestId('app-panel-header').querySelector('.am-rhs-app-ic img')).toHaveAttribute('src', '/icon.svg');
    });

    test('draws a neutral app icon for an app without one in the app rail', () => {
        renderWithContext(<AppPanelHeader pluginId='bare'/>, stateFor('bare'));
        expect(screen.getByTestId('app-panel-header').querySelector('.am-rhs-app-ic use')).toHaveAttribute('href', '#am-i-plug');
    });

    test('its buttons do what they say, with tooltips that flip below at the top of the window', async () => {
        const onRestoreSize = jest.fn();
        const onNewWindow = jest.fn();
        const onClose = jest.fn();
        renderWithContext(
            <AppPanelHeader
                pluginId='bare'
                sized={true}
                onRestoreSize={onRestoreSize}
                onNewWindow={onNewWindow}
                onClose={onClose}
            />,
            stateFor('bare'),
        );

        await userEvent.click(screen.getByRole('button', {name: "Restore the panel's size"}));
        await userEvent.click(screen.getByRole('button', {name: 'Open in a new window'}));
        await userEvent.click(screen.getByRole('button', {name: 'Close'}));
        expect(onRestoreSize).toHaveBeenCalledTimes(1);
        expect(onNewWindow).toHaveBeenCalledTimes(1);
        expect(onClose).toHaveBeenCalledTimes(1);

        // Fusion's tooltips (data-am-tip), which open below when there's no room above (useFaceTips).
        for (const button of screen.getAllByRole('button')) {
            expect(button).toHaveAttribute('data-am-tip', button.getAttribute('aria-label'));
            expect(button).not.toHaveAttribute('title');
        }
    });

    test('in its own window, the panel has only its icon and name', () => {
        renderWithContext(<AppPanelHeader pluginId='bare'/>, stateFor('bare'));
        expect(parts()).toEqual(['icon', 'name']);
    });

    describe('restoring the size', () => {
        beforeEach(() => {
            jest.spyOn(HTMLElement.prototype, 'getBoundingClientRect').mockImplementation(function(this: HTMLElement) {
                const width = parseFloat(this.dataset.width || '') || parseFloat(this.style.minWidth) || 0;
                return {width, height: 0, top: 0, left: 0, right: width, bottom: 0, x: 0, y: 0, toJSON: () => ({})} as DOMRect;
            });
        });
        afterEach(() => jest.restoreAllMocks());

        // The right-hand slot as it uses the header: its width, its resize handle, the header.
        function Panel() {
            const ref = useRef<HTMLDivElement>(null);
            const panel = usePanelWidth(ref, true);
            return (
                <div
                    ref={ref}
                    data-testid='panel'
                    data-width={panel.width ?? ''}
                    style={{minWidth: '400px', maxWidth: '800px'}}
                >
                    <PanelResizeHandle panel={panel}/>
                    <AppPanelHeader
                        pluginId='bare'
                        sized={panel.width !== null && panel.bounds !== null}
                        onRestoreSize={panel.reset}
                        onClose={jest.fn()}
                    />
                </div>
            );
        }
        const restore = () => screen.queryByRole('button', {name: "Restore the panel's size"});
        const width = () => screen.getByTestId('panel').dataset.width;

        test('sets the width back to the default and hides', async () => {
            renderWithContext(<Panel/>, {...stateFor('bare'), views: {rhs: {isSidebarExpanded: false}}} as never);
            expect(restore()).toBeNull();

            screen.getByRole('separator').focus();
            await userEvent.keyboard('{End}');
            expect(width()).toBe('800');
            expect(parts()).toEqual(['icon', 'name', "Restore the panel's size", 'Close']);

            await userEvent.click(restore()!);
            expect(width()).toBe('');
            expect(restore()).toBeNull();
        });

        test('hides once the panel is dragged back to its default width', () => {
            renderWithContext(<Panel/>, {...stateFor('bare'), views: {rhs: {isSidebarExpanded: false}}} as never);
            const handle = screen.getByRole('separator');

            fireEvent.pointerDown(handle, {button: 0, clientX: 1000, pointerId: 1});
            fireEvent.pointerMove(handle, {clientX: 900, pointerId: 1});
            expect(restore()).not.toBeNull();

            fireEvent.pointerMove(handle, {clientX: 1200, pointerId: 1});
            fireEvent.pointerUp(handle, {pointerId: 1});
            expect(width()).toBe('');
            expect(restore()).toBeNull();
        });
    });

    // The header is the same for every app: no app's id or name in its code, nor in the panel around it.
    test.each(['app_panel_header.tsx', '../../plugins/rhs_plugin/rhs_plugin.tsx', '../../components/rhs_plugin_popout/rhs_plugin_popout.tsx'])('%s has no app-specific code', (file) => {
        const source = fs.readFileSync(path.join(__dirname, file), 'utf8').toLowerCase();
        const apps = [
            'playbook', 'focalboard', 'kanban', 'boards', 'notes', 'todo', 'whiteboard', 'calendar', 'mail', 'polls',
            'memes', 'gifs', 'voice', 'agents', 'mattermost-ai', 'antimatterchat', 'calls', 'jira', 'github', 'gitlab',
        ];
        for (const app of apps) {
            expect({file, app, found: new RegExp(`\\b${app}\\b`).test(source)}).toEqual({file, app, found: false});
        }
        expect(source).not.toMatch(/pluginid\s*===?\s*['"]/);
    });
});
