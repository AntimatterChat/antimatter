// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import {RecordingBadge, RecordingControl} from './recording';

const mockActions = {toggleRecording: jest.fn(() => Promise.resolve())};
let mockCall: {threadId: string; recording: boolean} | null = null;
let mockVoice = false;
let mockAvailable = true;
jest.mock('./actions', () => ({useCallActions: () => mockActions}));
jest.mock('./hooks', () => ({
    useCall: () => mockCall,
    useIsVoiceChannel: () => mockVoice,
}));
jest.mock('./host_api', () => ({isRecordingAvailable: () => mockAvailable}));

describe('fusion/calls/recording', () => {
    beforeEach(() => {
        mockCall = {threadId: 'thread', recording: false};
        mockVoice = false;
        mockAvailable = true;
        mockActions.toggleRecording.mockClear();
    });

    test('the host records the call, and stops recording', async () => {
        const {rerender} = renderWithContext(
            <RecordingControl
                channelId='ch'
                isHost={true}
            />,
        );
        await userEvent.click(screen.getByRole('button', {name: 'Record the call'}));
        expect(mockActions.toggleRecording).toHaveBeenCalledWith('ch', false);

        mockCall = {threadId: 'thread', recording: true};
        rerender(
            <RecordingControl
                channelId='ch'
                isHost={true}
            />,
        );
        const stop = screen.getByRole('button', {name: 'Stop recording'});
        expect(stop).toHaveAttribute('aria-pressed', 'true');
        await userEvent.click(stop);
        expect(mockActions.toggleRecording).toHaveBeenCalledWith('ch', true);
    });

    test('only the host, where recordings are enabled, and in calls with a thread', () => {
        const {rerender} = renderWithContext(
            <RecordingControl
                channelId='ch'
                isHost={false}
            />,
        );
        expect(screen.queryByRole('button')).not.toBeInTheDocument();

        mockAvailable = false;
        rerender(
            <RecordingControl
                channelId='ch'
                isHost={true}
            />,
        );
        expect(screen.queryByRole('button')).not.toBeInTheDocument();

        // Voice channels' calls have no thread to post the recording in.
        mockAvailable = true;
        mockVoice = true;
        rerender(
            <RecordingControl
                channelId='ch'
                isHost={true}
            />,
        );
        expect(screen.queryByRole('button')).not.toBeInTheDocument();

        mockVoice = false;
        mockCall = {threadId: '', recording: false};
        rerender(
            <RecordingControl
                channelId='ch'
                isHost={true}
            />,
        );
        expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });

    test('everyone in the call sees that it is recorded', () => {
        const {rerender} = renderWithContext(<RecordingBadge channelId='ch'/>);
        expect(screen.queryByText('Recording')).not.toBeInTheDocument();

        mockCall = {threadId: 'thread', recording: true};
        rerender(<RecordingBadge channelId='ch'/>);
        expect(screen.getByText('Recording')).toBeInTheDocument();
    });
});
