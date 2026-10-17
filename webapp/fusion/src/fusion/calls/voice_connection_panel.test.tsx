// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import {goToChannel, showCall} from './actions';
import type {MyCallState} from './hooks';
import VoiceConnectionPanel from './voice_connection_panel';

const mockActions = {
    toggleMute: jest.fn(),
    toggleDeafen: jest.fn(),
    toggleVideo: jest.fn(),
    toggleScreen: jest.fn(),
    leaveCall: jest.fn(),
};
let mockMy: MyCallState | null = null;
const mockSetNavOpen = jest.fn();
jest.mock('./actions', () => ({
    useCallActions: () => mockActions,
    useChannelLabel: () => 'Standup',
    goToChannel: jest.fn(),
    showCall: jest.fn(),
}));
jest.mock('./hooks', () => ({
    useMyCall: () => mockMy,
    useParticipants: () => [],
    useCallMedia: () => ({}),
    useCall: () => null,
}));
jest.mock('./call_status', () => ({useCallStatus: () => 'connected'}));
jest.mock('./transcription', () => ({TranscriptionControls: () => null}));
jest.mock('./recording', () => ({RecordingControl: () => null}));
jest.mock('fusion/shell/layout_context', () => ({useLayout: () => ({setNavOpen: mockSetNavOpen})}));

function myCall(voice = false): MyCallState {
    return {
        call: {channelId: 'ch1', mediaInThisWindow: true},
        voice,
        muted: true,
        deafened: false,
        video: false,
        screen: false,
        handRaised: false,
        isHost: false,
        modMuted: false,
        otherSharing: false,
        videoAllowed: true,
        screenAllowed: true,
    } as unknown as MyCallState;
}

describe('fusion/calls/VoiceConnectionPanel', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        mockMy = null;
    });

    test('nothing while not in a call', () => {
        renderWithContext(<VoiceConnectionPanel/>);
        expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    // The channel header has no button to show the call any more: this one, at the bottom of the sidebar, is the way.
    test('shows the call in the main view, closing the sidebar drawer', async () => {
        mockMy = myCall();
        renderWithContext(<VoiceConnectionPanel/>);
        await userEvent.click(screen.getByRole('button', {name: 'Show the call'}));
        expect(mockSetNavOpen).toHaveBeenCalledWith(false);
        expect(showCall).toHaveBeenCalledWith(expect.anything(), 'ch1');
    });

    test('a voice channel shows itself the same way', async () => {
        mockMy = myCall(true);
        renderWithContext(<VoiceConnectionPanel/>);
        await userEvent.click(screen.getByRole('button', {name: 'Show the voice channel'}));
        expect(showCall).toHaveBeenCalledWith(expect.anything(), 'ch1');
    });

    test('goes to the call\'s channel, and disconnects', async () => {
        mockMy = myCall();
        renderWithContext(<VoiceConnectionPanel/>);
        await userEvent.click(screen.getByRole('button', {name: /^Standup · /}));
        expect(goToChannel).toHaveBeenCalledWith(expect.anything(), 'ch1');
        await userEvent.click(screen.getByRole('button', {name: 'Disconnect'}));
        expect(mockActions.leaveCall).toHaveBeenCalled();
    });
});
