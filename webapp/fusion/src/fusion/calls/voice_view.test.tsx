// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import type {MyCallState} from './hooks';

import VoiceView from './voice_view';

const mockJoinVoice = jest.fn();
jest.mock('./actions', () => ({
    useCallActions: () => ({joinVoice: mockJoinVoice}),
}));

let mockMy: MyCallState | null = null;
let mockParticipantCount = 0;
jest.mock('./hooks', () => ({
    useMyCall: () => mockMy,
    useParticipants: () => Array.from({length: mockParticipantCount}, (_, i) => ({sessionId: `s${i}`})),
}));

jest.mock('./stage', () => ({
    __esModule: true,
    default: ({channelId, fullscreen}: {channelId: string; fullscreen: boolean}) => (
        <div
            data-testid='stage'
            data-channel-id={channelId}
            data-fullscreen={String(fullscreen)}
        />
    ),
}));

jest.mock('./controls', () => ({
    __esModule: true,
    default: ({withPopout}: {withPopout: boolean}) => (
        <div
            data-testid='call-controls'
            data-with-popout={String(withPopout)}
        />
    ),
}));

// VoiceView is a voice channel's channel view (design CHANNEL_TYPES_DESIGN.md §4.4, Talk: "the call is the
// channel view"): the call's stage, with a Join voice button before you're in it, or your controls once you are.
describe('fusion/calls/VoiceView', () => {
    const channel = TestHelper.getChannelMock({id: 'voice', type: 'O', team_id: 'team'});

    beforeEach(() => {
        mockJoinVoice.mockClear();
        mockMy = null;
        mockParticipantCount = 0;
    });

    test('not joined: the stage, and a Join voice button that joins the call', async () => {
        renderWithContext(<VoiceView channel={channel}/>);

        const stage = screen.getByTestId('stage');
        expect(stage).toHaveAttribute('data-channel-id', 'voice');
        expect(stage).toHaveAttribute('data-fullscreen', 'true');
        expect(screen.queryByTestId('call-controls')).not.toBeInTheDocument();

        const button = screen.getByRole('button', {name: 'Join voice'});
        await userEvent.click(button);
        expect(mockJoinVoice).toHaveBeenCalledWith('voice');
    });

    test('not joined, people already in the call: the button says how many', () => {
        mockParticipantCount = 3;
        renderWithContext(<VoiceView channel={channel}/>);

        expect(screen.getByRole('button', {name: 'Join voice · 3 people in the call'})).toBeInTheDocument();
    });

    test('joined: the call controls instead of the join button', () => {
        mockMy = {
            call: {channelId: 'voice'} as MyCallState['call'],
            voice: true,
            muted: false,
            deafened: false,
            video: false,
            screen: false,
            handRaised: false,
            isHost: false,
            modMuted: false,
            otherSharing: false,
            videoAllowed: true,
            screenAllowed: true,
        };

        renderWithContext(<VoiceView channel={channel}/>);

        expect(screen.getByTestId('call-controls')).toHaveAttribute('data-with-popout', 'true');
        expect(screen.queryByRole('button', {name: /Join voice/})).not.toBeInTheDocument();
    });

    test('in a call, but of another channel: still shows the join button', () => {
        mockMy = {
            call: {channelId: 'other'} as MyCallState['call'],
            voice: true,
            muted: false,
            deafened: false,
            video: false,
            screen: false,
            handRaised: false,
            isHost: false,
            modMuted: false,
            otherSharing: false,
            videoAllowed: true,
            screenAllowed: true,
        };

        renderWithContext(<VoiceView channel={channel}/>);

        expect(screen.getByRole('button', {name: 'Join voice'})).toBeInTheDocument();
        expect(screen.queryByTestId('call-controls')).not.toBeInTheDocument();
    });
});
