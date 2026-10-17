// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {onSubmit} from 'actions/views/create_comment';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import Composer from './composer';

const mockTomorrow = Date.UTC(2030, 0, 1, 9);

jest.mock('actions/views/create_comment', () => ({
    ...jest.requireActual('actions/views/create_comment'),
    onSubmit: jest.fn(() => () => Promise.resolve({data: true})),
}));
jest.mock('actions/global_actions', () => ({emitLocalUserTypingEvent: jest.fn()}));
jest.mock('./confirm_mentions', () => ({confirmChannelWideMentions: () => () => Promise.resolve({data: true})}));
jest.mock('fusion/shell/dialogs_context', () => ({useDialogs: () => ({})}));
jest.mock('fusion/shell/toast_context', () => ({useToast: () => jest.fn()}));
jest.mock('components/common/hooks/useCreateBurnOnReadAccess', () => ({useCreateBurnOnReadAccess: () => false}));
jest.mock('./ai_rewrite', () => ({__esModule: true, default: () => null, useAIRewriteAvailable: () => false}));
jest.mock('./autocomplete', () => ({__esModule: true, default: jest.requireActual('react').forwardRef(() => null)}));
jest.mock('./away_note', () => ({__esModule: true, default: () => null}));
jest.mock('./sleep_note', () => ({__esModule: true, default: () => null}));
jest.mock('./typing', () => ({__esModule: true, default: () => null}));

// The schedule picker, reduced to one of its choices.
jest.mock('./schedule', () => ({
    ScheduledNote: () => null,
    SchedulePopover: ({onSchedule}: {onSchedule: (at: number) => void}) => (
        <button
            type='button'
            onClick={() => onSchedule(mockTomorrow)}
        >
            {'Tomorrow at 9:00'}
        </button>
    ),
}));

describe('fusion/composer/Composer', () => {
    const channel = TestHelper.getChannelMock({id: 'c1', name: 'general', display_name: 'General', type: 'O', team_id: 'team'});
    const render = () => renderWithContext(
        <Composer
            channelId='c1'
            placeholder='Write to General'
        />,
        {
            entities: {
                general: {config: {ScheduledPosts: 'true'}},
                users: {currentUserId: 'me'},
                channels: {currentChannelId: 'c1', channels: {c1: channel}},
            },
            plugins: {components: {}},
        } as never,
    );

    beforeEach(() => {
        jest.clearAllMocks();

        // The formatting bar shown, as chosen in this browser.
        localStorage.setItem('am-formatting', 'true');
    });
    afterEach(() => {
        localStorage.removeItem('am-formatting');
    });

    test('has no schedule button in the formatting bar: the send button\'s chevron schedules', () => {
        render();

        expect(screen.getByRole('toolbar', {name: 'Formatting and message options'})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Schedule message'})).not.toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Send later'})).toBeInTheDocument();
    });

    test('schedules the message from "Send later"', async () => {
        render();

        await userEvent.type(screen.getByRole('textbox', {name: 'Write to General'}), 'See you tomorrow');
        await userEvent.click(screen.getByRole('button', {name: 'Send later'}));
        await userEvent.click(screen.getByRole('button', {name: 'Tomorrow at 9:00'}));

        await waitFor(() => expect(onSubmit).toHaveBeenCalledWith('c1', '', expect.objectContaining({message: 'See you tomorrow'}), {}, {scheduled_at: mockTomorrow}));
    });
});
