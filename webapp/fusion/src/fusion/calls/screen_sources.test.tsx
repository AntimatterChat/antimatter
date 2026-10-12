// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {LAYER_ID} from 'fusion/components/layer/layer';
import {act, renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import type {AntimatterCallsAPI} from './plugin_api/calls_public_api';
import {screenPicker} from './screen_picker';
import {groupSources, ScreenSourcePicker, startScreenShare} from './screen_sources';

jest.mock('./screen_picker', () => ({screenPicker: jest.fn()}));

const SOURCES = [
    {id: 'window:7:0', name: 'Notes — a window with a very long title that does not fit under its thumbnail', thumbnailURL: 'data:image/png;base64,AA'},
    {id: 'screen:0:0', name: 'Entire screen', thumbnailURL: 'data:image/png;base64,AB'},
    {id: 'window:9:0', name: 'Calls Widget', thumbnailURL: 'data:image/png;base64,AC'},
];

function mockCalls(canShareSystemAudio = false) {
    const api = {
        version: 1,
        startScreenShare: jest.fn(),
        canShareSystemAudio: jest.fn(() => canShareSystemAudio),
    };
    window.antimatterCalls = api as unknown as AntimatterCallsAPI;
    return api;
}

describe('fusion/calls/screen_sources', () => {
    let layer: HTMLElement;
    let getDesktopSources: jest.Mock;
    beforeEach(() => {
        layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.append(layer);
        getDesktopSources = jest.fn(() => Promise.resolve(SOURCES));
        window.desktopAPI = {getDesktopSources};
    });
    afterEach(() => {
        layer.remove();
        delete window.antimatterCalls;
        delete window.desktopAPI;
    });

    test('groupSources puts the screens first and leaves the desktop app\'s call window out', () => {
        expect(groupSources(SOURCES)).toEqual({
            screens: [SOURCES[1]],
            windows: [SOURCES[0]],
        });
    });

    test('in a browser, sharing asks the browser right away', () => {
        jest.mocked(screenPicker).mockReturnValue('surfaces');
        const api = mockCalls();
        startScreenShare({height: 720, frameRate: 30}, 'window');
        expect(api.startScreenShare).toHaveBeenCalledWith({height: 720, frameRate: 30}, 'window');
    });

    test('in the desktop app, the picked source is shared at the quality asked for', async () => {
        jest.mocked(screenPicker).mockReturnValue('desktop');
        const api = mockCalls();
        renderWithContext(<ScreenSourcePicker/>);
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();

        act(() => startScreenShare({height: 1080, frameRate: 60}));
        expect(api.startScreenShare).not.toHaveBeenCalled();
        expect(screen.getByRole('dialog', {name: 'Choose what to share'})).toBeInTheDocument();
        expect(getDesktopSources).toHaveBeenCalledWith({types: ['screen', 'window'], thumbnailSize: {width: 400, height: 400}});

        // The screen is picked first; the call window isn't offered
        const screenTile = await screen.findByRole('radio', {name: 'Entire screen'});
        expect(screenTile).toHaveAttribute('aria-checked', 'true');
        expect(screen.queryByRole('radio', {name: 'Calls Widget'})).not.toBeInTheDocument();
        expect(screen.queryByRole('switch')).not.toBeInTheDocument();

        await userEvent.click(screen.getByRole('radio', {name: SOURCES[0].name}));
        await userEvent.click(screen.getByRole('button', {name: 'Share'}));
        expect(api.startScreenShare).toHaveBeenCalledWith({height: 1080, frameRate: 60}, undefined, {desktopSourceId: 'window:7:0', withAudio: false});
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
    });

    test('the system\'s sound can be shared too where Calls allows it', async () => {
        jest.mocked(screenPicker).mockReturnValue('desktop');
        const api = mockCalls(true);
        renderWithContext(<ScreenSourcePicker/>);
        act(() => startScreenShare());

        await userEvent.click(await screen.findByRole('switch', {name: 'Also share the computer\'s sound'}));
        await userEvent.dblClick(screen.getByRole('radio', {name: 'Entire screen'}));
        expect(api.startScreenShare).toHaveBeenCalledWith(undefined, undefined, {desktopSourceId: 'screen:0:0', withAudio: true});
    });

    test('cancelling shares nothing', async () => {
        jest.mocked(screenPicker).mockReturnValue('desktop');
        const api = mockCalls();
        renderWithContext(<ScreenSourcePicker/>);
        act(() => startScreenShare());

        await screen.findByRole('radio', {name: 'Entire screen'});
        await userEvent.click(screen.getByRole('button', {name: 'Cancel'}));
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(api.startScreenShare).not.toHaveBeenCalled();
    });

    test('the dialog closes when the sources can\'t be listed', async () => {
        jest.mocked(screenPicker).mockReturnValue('desktop');
        mockCalls();
        getDesktopSources.mockReturnValue(Promise.reject(new Error('permissions denied')));
        renderWithContext(<ScreenSourcePicker/>);
        act(() => startScreenShare());

        await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    });
});
