// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {LAYER_ID} from 'fusion/components/layer/layer';
import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import CallControls, {CameraMenu, ScreenShareMenu} from './controls';
import {getScreenSurface} from './extras';
import type {MyCallState} from './hooks';
import type {AntimatterCallsAPI} from './plugin_api/calls_public_api';
import {screenPicker} from './screen_picker';

jest.mock('./screen_picker', () => ({screenPicker: jest.fn()}));

const mockActions = {
    toggleMute: jest.fn(),
    toggleDeafen: jest.fn(),
    toggleVideo: jest.fn(),
    toggleScreen: jest.fn(),
    toggleHand: jest.fn(),
    leaveCall: jest.fn(),
};
jest.mock('./actions', () => ({useCallActions: () => mockActions}));
jest.mock('./hooks', () => ({useCall: () => null}));
jest.mock('./transcription', () => ({TranscriptionControls: () => null}));

function mockCalls(blur = {on: false, intensity: 50}) {
    const api = {
        version: 1,
        startScreenShare: jest.fn(),
        getBackgroundBlur: jest.fn(() => blur),
        setBackgroundBlur: jest.fn(() => Promise.resolve()),
        openSettings: jest.fn(),
    };
    window.antimatterCalls = api as unknown as AntimatterCallsAPI;
    return api;
}

describe('fusion/calls/controls menus', () => {
    let anchor: HTMLElement;
    let layer: HTMLElement;
    beforeEach(() => {
        anchor = document.createElement('button');
        layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.append(anchor, layer);
    });
    afterEach(() => {
        anchor.remove();
        layer.remove();
        delete window.antimatterCalls;
    });

    test('in Chromium, a screen, a window or a tab is offered, and asked for', async () => {
        jest.mocked(screenPicker).mockReturnValue('surfaces');
        const api = mockCalls();
        const onClose = jest.fn();
        renderWithContext(
            <ScreenShareMenu
                anchor={anchor}
                onClose={onClose}
            />,
        );

        expect(screen.getByRole('menuitem', {name: 'A whole screen'})).toBeInTheDocument();
        expect(screen.getByRole('menuitem', {name: 'A window'})).toBeInTheDocument();
        await userEvent.click(screen.getByRole('menuitem', {name: 'A browser tab'}));
        expect(api.startScreenShare).toHaveBeenCalledWith({height: 1080, frameRate: 30}, 'browser');
        expect(onClose).toHaveBeenCalled();

        // The share button asks for it again.
        expect(getScreenSurface()).toBe('browser');
    });

    test.each(['browser', 'desktop'] as const)('with the %s picker, one item opens it, without asking for a kind of source', async (picker) => {
        jest.mocked(screenPicker).mockReturnValue(picker);
        const api = mockCalls();
        renderWithContext(
            <ScreenShareMenu
                anchor={anchor}
                onClose={jest.fn()}
            />,
        );

        expect(screen.getAllByRole('menuitem')).toHaveLength(1);
        expect(screen.queryByRole('menuitem', {name: 'A browser tab'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('menuitem', {name: 'Choose a screen or a window…'}));
        expect(api.startScreenShare).toHaveBeenCalledWith({height: 1080, frameRate: 30}, undefined);
    });

    test('the camera menu blurs the background, at a preset', async () => {
        const api = mockCalls();
        renderWithContext(
            <CameraMenu
                anchor={anchor}
                onClose={jest.fn()}
            />,
        );

        expect(screen.getByText('Camera')).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Strong'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('menuitemcheckbox', {name: 'Blur my background'}));
        expect(api.setBackgroundBlur).toHaveBeenLastCalledWith({on: true, intensity: 50});
        expect(screen.getByRole('menuitemcheckbox', {name: 'Blur my background'})).toHaveAttribute('aria-checked', 'true');
        expect(screen.getByRole('button', {name: 'Medium'})).toHaveAttribute('aria-pressed', 'true');

        await userEvent.click(screen.getByRole('button', {name: 'Strong'}));
        expect(api.setBackgroundBlur).toHaveBeenLastCalledWith({on: true, intensity: 100});
        expect(screen.getByRole('button', {name: 'Strong'})).toHaveAttribute('aria-pressed', 'true');
    });

    test('the camera menu opens the device settings', async () => {
        const api = mockCalls({on: true, intensity: 70});
        const onClose = jest.fn();
        renderWithContext(
            <CameraMenu
                anchor={anchor}
                onClose={onClose}
            />,
        );

        // A blur set with the older slider shows as its nearest preset.
        expect(screen.getByRole('button', {name: 'Medium'})).toHaveAttribute('aria-pressed', 'true');
        await userEvent.click(screen.getByRole('menuitem', {name: 'Camera and microphone settings…'}));
        expect(api.openSettings).toHaveBeenCalled();
        expect(onClose).toHaveBeenCalled();
    });
});

describe('fusion/calls/controls CallControls', () => {
    let layer: HTMLElement;
    beforeEach(() => {
        layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.append(layer);
        jest.mocked(screenPicker).mockReturnValue('surfaces');
        Object.values(mockActions).forEach((fn) => fn.mockClear());
    });
    afterEach(() => {
        layer.remove();
        delete window.antimatterCalls;
    });

    const myCall = (over: Partial<MyCallState> = {}): MyCallState => ({
        call: {channelId: 'channel1', mediaInThisWindow: true} as MyCallState['call'],
        voice: false,
        muted: false,
        deafened: false,
        video: false,
        screen: false,
        handRaised: false,
        isHost: false,
        otherSharing: false,
        videoAllowed: true,
        screenAllowed: true,
        ...over,
    });

    test('the camera button turns the camera on; its badge opens its options', async () => {
        mockCalls();
        const my = myCall();
        renderWithContext(<CallControls my={my}/>);

        await userEvent.click(screen.getByRole('button', {name: 'Camera'}));
        expect(mockActions.toggleVideo).toHaveBeenCalledWith(my);
        expect(screen.queryByRole('menu')).not.toBeInTheDocument();

        const badge = screen.getByRole('button', {name: 'Camera options'});
        expect(badge).toHaveAttribute('aria-haspopup', 'menu');
        await userEvent.click(badge);
        expect(screen.getByRole('menu', {name: 'Camera options'})).toBeInTheDocument();
        expect(badge).toHaveAttribute('aria-expanded', 'true');
        expect(mockActions.toggleVideo).toHaveBeenCalledTimes(1);

        await userEvent.click(badge);
        expect(screen.queryByRole('menu')).not.toBeInTheDocument();
    });

    test('the share button shares right away; its badge opens the share menu', async () => {
        const api = mockCalls();
        const my = myCall();
        renderWithContext(<CallControls my={my}/>);

        await userEvent.click(screen.getByRole('button', {name: 'Share screen'}));
        expect(mockActions.toggleScreen).toHaveBeenCalledWith(my);
        expect(screen.queryByRole('menu')).not.toBeInTheDocument();

        const badge = screen.getByRole('button', {name: 'Screen share options'});
        expect(badge).toHaveAttribute('aria-haspopup', 'menu');
        await userEvent.click(badge);
        expect(screen.getByRole('menu', {name: 'Share your screen'})).toBeInTheDocument();
        await userEvent.click(screen.getByRole('menuitem', {name: 'A window'}));
        expect(api.startScreenShare).toHaveBeenCalledWith({height: 1080, frameRate: 30}, 'window');
        expect(mockActions.toggleScreen).toHaveBeenCalledTimes(1);
    });

    test('while sharing, the share button stops it and has no options', async () => {
        mockCalls();
        const my = myCall({screen: true});
        renderWithContext(<CallControls my={my}/>);

        await userEvent.click(screen.getByRole('button', {name: 'Stop sharing'}));
        expect(mockActions.toggleScreen).toHaveBeenCalledWith(my);
        expect(screen.queryByRole('button', {name: 'Screen share options'})).not.toBeInTheDocument();
    });

    test('when someone else shares, there are no share options either', () => {
        mockCalls();
        renderWithContext(<CallControls my={myCall({otherSharing: true})}/>);

        expect(screen.getByRole('button', {name: 'Someone else is sharing their screen'})).toBeDisabled();
        expect(screen.queryByRole('button', {name: 'Screen share options'})).not.toBeInTheDocument();
    });
});
