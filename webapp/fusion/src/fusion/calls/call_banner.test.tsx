// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import CallBanner from './call_banner';

const mockActions = {joinAndShowCall: jest.fn()};
let mockCall: {ownerId: string; startAt: number; title?: string} | undefined;
let mockLocal: {channelId: string} | null = null;
let mockVoice = false;
let mockParticipants: Array<{sessionId: string; userId: string}> = [];
jest.mock('./actions', () => ({useCallActions: () => mockActions, firstName: () => 'Kenji'}));
jest.mock('./hooks', () => ({
    useCall: () => mockCall,
    useIsVoiceChannel: () => mockVoice,
    useLocalCall: () => mockLocal,
    useParticipants: () => mockParticipants,
}));
jest.mock('fusion/components/avatar', () => () => null);

const channel = TestHelper.getChannelMock({id: 'c1', delete_at: 0});

describe('fusion/calls/CallBanner', () => {
    beforeEach(() => {
        mockCall = {ownerId: 'kenji', startAt: Date.now() - 65000};
        mockLocal = null;
        mockVoice = false;
        mockParticipants = [{sessionId: 's1', userId: 'kenji'}, {sessionId: 's2', userId: 'ana'}];
        mockActions.joinAndShowCall.mockClear();
    });

    test('tells who started the call and how many are in it, and joins it', async () => {
        renderWithContext(<CallBanner channel={channel}/>);
        const banner = screen.getByRole('status');
        expect(banner).toHaveTextContent('Kenji started a call');
        expect(banner).toHaveTextContent('2 people in it');
        expect(banner).toHaveTextContent(/1:0\d/);
        await userEvent.click(screen.getByRole('button', {name: 'Join call'}));
        expect(mockActions.joinAndShowCall).toHaveBeenCalledWith('c1');
    });

    test('names a titled call', () => {
        mockCall = {ownerId: 'kenji', startAt: Date.now(), title: 'Standup'};
        mockParticipants = [];
        renderWithContext(<CallBanner channel={channel}/>);
        expect(screen.getByRole('status')).toHaveTextContent('“Standup” is in progress');
        expect(screen.getByRole('status')).toHaveTextContent('Nobody in it yet');
    });

    test('shows nothing without a call, in the call, in voice channels or archived channels', () => {
        mockCall = undefined;
        const {rerender} = renderWithContext(<CallBanner channel={channel}/>);
        expect(screen.queryByRole('status')).not.toBeInTheDocument();

        mockCall = {ownerId: 'kenji', startAt: 1};
        mockLocal = {channelId: 'c1'};
        rerender(<CallBanner channel={channel}/>);
        expect(screen.queryByRole('status')).not.toBeInTheDocument();

        mockLocal = null;
        mockVoice = true;
        rerender(<CallBanner channel={channel}/>);
        expect(screen.queryByRole('status')).not.toBeInTheDocument();

        mockVoice = false;
        rerender(<CallBanner channel={{...channel, delete_at: 9}}/>);
        expect(screen.queryByRole('status')).not.toBeInTheDocument();
    });
});
