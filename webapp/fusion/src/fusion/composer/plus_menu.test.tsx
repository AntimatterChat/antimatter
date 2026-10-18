// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent, within} from 'tests/react_testing_utils';

import PlusMenu from './plus_menu';

describe('fusion/composer/PlusMenu', () => {
    beforeEach(() => {
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);
    });
    afterEach(() => {
        document.getElementById(LAYER_ID)?.remove();
    });

    const render = (state: Record<string, unknown> = {}) => {
        const props = {
            anchor: null,
            channelId: 'c1',
            rootId: '',
            onPickFile: jest.fn(),
            onUpload: jest.fn(),
            onClose: jest.fn(),
        };
        renderWithContext(<PlusMenu {...props}/>, {plugins: {components: {}}, ...state} as never);
        return props;
    };

    test('only adds things to the message: no scheduling, no slash command', () => {
        render();
        const menu = screen.getByRole('menu', {name: 'Add to message'});
        expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual(['Upload a file']);
        expect(screen.queryByText('Schedule message')).not.toBeInTheDocument();
        expect(screen.queryByText('Use a slash command')).not.toBeInTheDocument();
    });

    test('uploads a file', async () => {
        const props = render();
        await userEvent.click(screen.getByRole('menuitem', {name: 'Upload a file'}));
        expect(props.onClose).toHaveBeenCalled();
        expect(props.onPickFile).toHaveBeenCalled();
    });

    test('lists what apps add: upload methods and composer items', async () => {
        const files = [new File(['x'], 'x.png')];
        const method = {id: 'm1', pluginId: 'gifs', text: 'Add a GIF', icon: <i/>, action: jest.fn((done: (f: File[]) => void) => done(files))};
        const poll = {id: 'poll', pluginId: 'polls', text: 'Create a poll', icon: <i/>, action: jest.fn(), shouldRender: () => true};
        const props = render({plugins: {components: {FileUploadMethod: [method], ComposerMenuItem: [poll]}}});

        const menu = screen.getByRole('menu', {name: 'Add to message'});
        expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual(['Upload a file', 'Add a GIF', 'Create a poll']);

        await userEvent.click(screen.getByRole('menuitem', {name: 'Add a GIF'}));
        expect(props.onUpload).toHaveBeenCalledWith(files);
    });

    test('offers the apps\' composer modes, which the composer switches to', async () => {
        const onMode = jest.fn();
        const onClose = jest.fn();
        const poll = {id: 'polls.poll', pluginId: 'polls', label: 'Create a poll', fusionIcon: 'poll', component: () => null, shouldRender: () => true};
        renderWithContext(
            <PlusMenu
                anchor={null}
                channelId='c1'
                rootId=''
                onPickFile={jest.fn()}
                onUpload={jest.fn()}
                onClose={onClose}
                modes={[poll]}
                onMode={onMode}
            />,
            {plugins: {components: {}}} as never,
        );
        const menu = screen.getByRole('menu', {name: 'Add to message'});
        expect(within(menu).getAllByRole('menuitem').map((i) => i.textContent)).toEqual(['Upload a file', 'Create a poll']);

        await userEvent.click(screen.getByRole('menuitem', {name: 'Create a poll'}));
        expect(onClose).toHaveBeenCalled();
        expect(onMode).toHaveBeenCalledWith('polls.poll');
    });

    test('in a narrow composer, it also opens the message options', async () => {
        const onOptions = jest.fn();
        renderWithContext(
            <PlusMenu
                anchor={null}
                channelId='c1'
                rootId=''
                onPickFile={jest.fn()}
                onUpload={jest.fn()}
                onClose={jest.fn()}
                onOptions={onOptions}
            />,
            {plugins: {components: {}}} as never,
        );
        await userEvent.click(screen.getByRole('menuitem', {name: 'Message options'}));
        expect(onOptions).toHaveBeenCalled();
    });
});
