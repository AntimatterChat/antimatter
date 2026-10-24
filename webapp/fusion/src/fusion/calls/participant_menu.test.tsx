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
    modDeafen: jest.fn(),
    moveTo: jest.fn(),
    askToUnmute: jest.fn(),
    openDirect: jest.fn(),
    callUser: jest.fn(),
    makeHost: jest.fn(),
};
let mockModerate = true;
let mockMakeHost = false;
let mockVoice = true;
let mockTargets: Array<{id: string; display_name: string}> = [];
jest.mock('./actions', () => ({
    useCallActions: () => mockActions,
    useChannelLabel: () => 'Lobby',
    goToChannel: jest.fn(),
}));
jest.mock('./hooks', () => ({
    useCanModerateCall: () => mockModerate,
    useCanMakeHost: () => mockMakeHost,
    useIsVoiceChannel: () => mockVoice,
    useMoveTargets: (channelId: string) => (channelId ? mockTargets : []),
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
    modDeafened: false,
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
        mockVoice = true;
        mockTargets = [];
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
        mockVoice = false;
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
        mockTargets = [{id: 'v2', display_name: 'Gaming'}];
        window.antimatterCalls = {version: 1, moderation: {setModDeafened: jest.fn(), move: jest.fn()}} as unknown as AntimatterCallsAPI;
        open(participant({muted: true}));
        expect(screen.queryByRole('menuitem', {name: 'Ask to unmute'})).not.toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: /Mute in this channel/})).not.toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: /Deafen/})).not.toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: 'Move to'})).not.toBeInTheDocument();
    });

    test('older Calls versions offer the host controls they have', () => {
        window.antimatterCalls = {version: 1} as unknown as AntimatterCallsAPI;
        mockTargets = [{id: 'v2', display_name: 'Gaming'}];
        open(participant({muted: true}));
        expect(screen.queryByRole('menuitem', {name: 'Ask to unmute'})).not.toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: /Mute in this channel/})).not.toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: /Deafen/})).not.toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: 'Move to'})).not.toBeInTheDocument();
        expect(screen.getByRole('menuitem', {name: 'Kick from voice channel'})).toBeInTheDocument();
    });

    describe('in a voice channel', () => {
        beforeEach(() => {
            window.antimatterCalls = {version: 1, moderation: {setModDeafened: jest.fn(), move: jest.fn()}} as unknown as AntimatterCallsAPI;
        });

        test('the moderator mute lasts in the channel', async () => {
            open(participant());
            expect(screen.queryByRole('menuitem', {name: /Mute for this call/})).not.toBeInTheDocument();
            await userEvent.click(screen.getByRole('menuitem', {name: /Mute in this channel/}));
            expect(mockActions.modMute).toHaveBeenCalledWith(expect.objectContaining({sessionId: 's1'}), 'room', true);
        });

        test('a moderator deafens someone, and undeafens them', async () => {
            open(participant());
            await userEvent.click(screen.getByRole('menuitem', {name: /^Deafen/}));
            expect(mockActions.modDeafen).toHaveBeenCalledWith(expect.objectContaining({userId: 'u1'}), 'room', true);
        });

        test('someone deafened by a moderator is undeafened, not asked to unmute', async () => {
            open(participant({muted: true, deafened: true, modDeafened: true}));
            expect(screen.queryByRole('menuitem', {name: 'Ask to unmute'})).not.toBeInTheDocument();
            await userEvent.click(screen.getByRole('menuitem', {name: 'Undeafen'}));
            expect(mockActions.modDeafen).toHaveBeenCalledWith(expect.objectContaining({userId: 'u1'}), 'room', false);
        });

        test('a moderator moves someone to another voice channel, from the menu that also disconnects them', async () => {
            mockTargets = [{id: 'v2', display_name: 'Gaming'}, {id: 'v3', display_name: 'Music'}];
            open(participant());
            expect(screen.getByRole('menuitem', {name: 'Move to'})).toBeInTheDocument();
            expect(screen.getByRole('menuitem', {name: 'Kick from voice channel'})).toBeInTheDocument();
            await userEvent.click(screen.getByRole('menuitem', {name: 'Music'}));
            expect(mockActions.moveTo).toHaveBeenCalledWith(expect.objectContaining({sessionId: 's1'}), 'room', 'v3');
        });

        test('there\'s nowhere to move to without other voice channels', () => {
            open(participant());
            expect(screen.queryByRole('menuitem', {name: 'Move to'})).not.toBeInTheDocument();
        });
    });

    test('outside voice channels, nobody is deafened or moved', () => {
        mockVoice = false;
        mockTargets = [{id: 'v2', display_name: 'Gaming'}];
        window.antimatterCalls = {version: 1, moderation: {setModDeafened: jest.fn(), move: jest.fn()}} as unknown as AntimatterCallsAPI;
        open(participant());
        expect(screen.queryByRole('menuitem', {name: /Deafen/})).not.toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: 'Move to'})).not.toBeInTheDocument();
        expect(screen.getByRole('menuitem', {name: 'Remove from call'})).toBeInTheDocument();
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
