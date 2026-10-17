// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {LAYER_ID} from 'fusion/components/layer/layer';
import {fireEvent, renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import type {VoiceParticipant} from './calls_api';
import {useParticipantMenu} from './participant_menu';
import type {AntimatterCallsAPI} from './plugin_api/calls_public_api';

const mockActions = {
    hostMute: jest.fn(),
    hostRemove: jest.fn(),
    hostLowerHand: jest.fn(),
    hostStopScreen: jest.fn(),
    modMute: jest.fn(),
    askToUnmute: jest.fn(),
    openDirect: jest.fn(),
    callUser: jest.fn(),
    makeHost: jest.fn(),
};
let mockModerate = true;
let mockMakeHost = false;
jest.mock('./actions', () => ({
    useCallActions: () => mockActions,
    useChannelLabel: () => 'Lobby',
    goToChannel: jest.fn(),
}));
jest.mock('./hooks', () => ({
    useCanModerateCall: () => mockModerate,
    useCanMakeHost: () => mockMakeHost,
    useIsVoiceChannel: () => true,
}));
jest.mock('./extras', () => ({canSetVolume: () => false, getUserVolume: () => 1, setUserVolume: jest.fn()}));

const participant = (over: Partial<VoiceParticipant> = {}): VoiceParticipant => ({
    sessionId: 's1',
    userId: 'u1',
    muted: false,
    deafened: false,
    video: false,
    screenSharing: false,
    speaking: false,
    raisedHand: false,
    isHost: false,
    isMe: false,
    modMuted: false,
    ...over,
});

function Harness({p}: {p: VoiceParticipant}) {
    const menu = useParticipantMenu('room');
    return (
        <>
            <div
                data-testid='tile'
                onContextMenu={(e) => menu.open(p, e)}
            />
            {menu.element}
        </>
    );
}

describe('fusion/calls/participant_menu', () => {
    let layer: HTMLElement;
    beforeEach(() => {
        layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.append(layer);
        mockModerate = true;
        mockMakeHost = false;
        Object.values(mockActions).forEach((fn) => fn.mockClear());
        window.antimatterCalls = {version: 1, moderation: {}} as unknown as AntimatterCallsAPI;
    });
    afterEach(() => {
        layer.remove();
        delete window.antimatterCalls;
    });

    const open = (p: VoiceParticipant) => {
        renderWithContext(<Harness p={p}/>);
        fireEvent.contextMenu(screen.getByTestId('tile'));
    };

    test('a moderator mutes someone, or mutes them for the call', async () => {
        open(participant());
        expect(screen.queryByRole('menuitem', {name: 'Ask to unmute'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('menuitem', {name: 'Mute'}));
        expect(mockActions.hostMute).toHaveBeenCalledWith(expect.objectContaining({sessionId: 's1'}), 'room');

        fireEvent.contextMenu(screen.getByTestId('tile'));
        await userEvent.click(screen.getByRole('menuitem', {name: /Mute for this call/}));
        expect(mockActions.modMute).toHaveBeenCalledWith(expect.objectContaining({sessionId: 's1'}), 'room', true);
    });

    test('someone muted can be asked to unmute', async () => {
        open(participant({muted: true}));
        expect(screen.queryByRole('menuitem', {name: 'Mute'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('menuitem', {name: 'Ask to unmute'}));
        expect(mockActions.askToUnmute).toHaveBeenCalledWith(expect.objectContaining({sessionId: 's1'}), 'room');
    });

    test('someone muted by a moderator can be allowed to unmute, not asked to', async () => {
        open(participant({muted: true, modMuted: true}));
        expect(screen.queryByRole('menuitem', {name: 'Ask to unmute'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('menuitem', {name: 'Allow to unmute'}));
        expect(mockActions.modMute).toHaveBeenCalledWith(expect.objectContaining({sessionId: 's1'}), 'room', false);
    });

    test('the host controls act on the call of the menu\'s channel', async () => {
        open(participant({raisedHand: true, screenSharing: true}));
        await userEvent.click(screen.getByRole('menuitem', {name: 'Lower hand'}));
        expect(mockActions.hostLowerHand).toHaveBeenCalledWith(expect.objectContaining({sessionId: 's1'}), 'room');
        fireEvent.contextMenu(screen.getByTestId('tile'));
        await userEvent.click(screen.getByRole('menuitem', {name: 'Stop screen share'}));
        expect(mockActions.hostStopScreen).toHaveBeenCalledWith(expect.objectContaining({sessionId: 's1'}), 'room');
        fireEvent.contextMenu(screen.getByTestId('tile'));
        await userEvent.click(screen.getByRole('menuitem', {name: 'Kick from voice channel'}));
        expect(mockActions.hostRemove).toHaveBeenCalledWith(expect.objectContaining({sessionId: 's1'}), 'room');
    });

    test('without moderation rights, or for yourself, there are no host controls', () => {
        mockModerate = false;
        open(participant({muted: true}));
        expect(screen.queryByRole('menuitem', {name: 'Ask to unmute'})).not.toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: /Mute for this call/})).not.toBeInTheDocument();
    });

    test('older Calls versions offer the host controls they have', () => {
        window.antimatterCalls = {version: 1} as unknown as AntimatterCallsAPI;
        open(participant({muted: true}));
        expect(screen.queryByRole('menuitem', {name: 'Ask to unmute'})).not.toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: /Mute for this call/})).not.toBeInTheDocument();
        expect(screen.getByRole('menuitem', {name: 'Kick from voice channel'})).toBeInTheDocument();
    });

    test('the host makes someone else the host', async () => {
        mockMakeHost = true;
        open(participant());
        await userEvent.click(screen.getByRole('menuitem', {name: /Make host/}));
        expect(mockActions.makeHost).toHaveBeenCalledWith(expect.objectContaining({userId: 'u1'}), 'room');
    });

    test('nobody is made host who is already, and only the host and system admins can', () => {
        mockMakeHost = true;
        open(participant({isHost: true}));
        expect(screen.queryByRole('menuitem', {name: /Make host/})).not.toBeInTheDocument();
    });

    test('a moderator who isn\'t the host can\'t make someone host', () => {
        mockMakeHost = false;
        open(participant());
        expect(screen.getByRole('menuitem', {name: 'Mute'})).toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: /Make host/})).not.toBeInTheDocument();
    });
});
