// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Post} from '@mattermost/types/posts';

import type {ConfirmOptions} from 'fusion/modals/confirm_dialog';
import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import CallCard from './call_card';

const mockActions = {startOrJoinCall: jest.fn(), leaveCall: jest.fn(), endForEveryone: jest.fn()};
const mockConfirm = jest.fn();
let mockLocal: {channelId: string} | null = null;
let mockCanEnd = false;
const startAt = Date.now() - 60000;
jest.mock('./actions', () => ({useCallActions: () => mockActions, useChannelLabel: () => '#town-square'}));
jest.mock('./hooks', () => ({
    useCall: () => ({id: 'call', channelId: 'ch', threadId: 'post', startAt, ownerId: 'starter', hostId: 'someone', recording: false}),
    useCallsAvailable: () => true,
    useCanEndCall: () => mockCanEnd,
    useLocalCall: () => mockLocal,
    useParticipants: () => [],
}));
jest.mock('fusion/shell/dialogs_context', () => ({useDialogs: () => ({confirm: mockConfirm})}));

describe('fusion/calls/call_card: ending a call for everyone', () => {
    const post = TestHelper.getPostMock({id: 'post', channel_id: 'ch', user_id: 'starter', type: 'custom_calls' as Post['type'], props: {start_at: startAt}});

    beforeEach(() => {
        mockLocal = null;
        mockCanEnd = false;
        jest.clearAllMocks();
    });

    test('once out of the call, someone allowed to end it can, after confirming', async () => {
        mockCanEnd = true;
        renderWithContext(<CallCard post={post}/>);
        expect(screen.getByRole('button', {name: 'Join'})).toBeInTheDocument();

        await userEvent.click(screen.getByRole('button', {name: 'End call for everyone'}));
        expect(mockActions.endForEveryone).not.toHaveBeenCalled();
        expect(mockConfirm).toHaveBeenCalledTimes(1);
        const options: ConfirmOptions = mockConfirm.mock.calls[0][0];
        expect(options.title).toBe('End the call for everyone?');
        expect(options.danger).toBe(true);

        options.onConfirm();
        expect(mockActions.endForEveryone).toHaveBeenCalledWith('ch');
    });

    test('in the call, the card only leaves it', async () => {
        mockCanEnd = true;
        mockLocal = {channelId: 'ch'};
        renderWithContext(<CallCard post={post}/>);
        expect(screen.queryByRole('button', {name: 'End call for everyone'})).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: 'Leave'}));
        expect(mockActions.leaveCall).toHaveBeenCalled();
        expect(mockActions.endForEveryone).not.toHaveBeenCalled();
    });

    test('someone not allowed to end the call only joins it', () => {
        renderWithContext(<CallCard post={post}/>);
        expect(screen.getByRole('button', {name: 'Join'})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'End call for everyone'})).not.toBeInTheDocument();
    });
});
