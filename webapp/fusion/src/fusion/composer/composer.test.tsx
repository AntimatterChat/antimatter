// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {onSubmit} from 'actions/views/create_comment';

import {LAYER_ID} from 'fusion/components/layer';
import {fireEvent, renderWithContext, screen, userEvent, waitFor, within} from 'tests/react_testing_utils';
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
let mockBurnAllowed = false;
jest.mock('components/common/hooks/useCreateBurnOnReadAccess', () => ({useCreateBurnOnReadAccess: () => mockBurnAllowed}));
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
    const render = (config: Record<string, string> = {}) => renderWithContext(
        <Composer
            channelId='c1'
            placeholder='Write to General'
        />,
        {
            entities: {
                general: {config: {ScheduledPosts: 'true', ...config}},
                users: {currentUserId: 'me'},
                channels: {currentChannelId: 'c1', channels: {c1: channel}},
            },
            plugins: {components: {}},
        } as never,
    );

    beforeEach(() => {
        jest.clearAllMocks();
        mockBurnAllowed = false;
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);

        // The formatting bar shown, as chosen in this browser.
        localStorage.setItem('am-formatting', 'true');
    });
    afterEach(() => {
        localStorage.removeItem('am-formatting');
        document.getElementById(LAYER_ID)?.remove();
    });

    test('has no schedule button in the formatting bar: the send button\'s chevron schedules', () => {
        render();

        expect(screen.getByRole('toolbar', {name: 'Formatting'})).toBeInTheDocument();
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

    describe('message options', () => {
        const options = {PostPriority: 'true', EnableBurnOnRead: 'true', BurnOnReadDurationSeconds: '600', BurnOnReadMaximumDurationSeconds: '14400', BurnOnReadMaximumTimeToLiveSeconds: '604800'};
        const open = () => userEvent.click(screen.getByRole('button', {name: 'Message options'}));
        const popover = () => screen.getByRole('dialog', {name: 'Message options'});
        const sent = () => jest.mocked(onSubmit).mock.calls[0][2];

        beforeEach(() => {
            mockBurnAllowed = true;
        });

        test('one button opens them all; the formatting bar no longer has them', async () => {
            render(options);
            const bar = screen.getByRole('toolbar', {name: 'Formatting'});
            expect(within(bar).queryByRole('button', {name: /priority|burn|spoiler/i})).not.toBeInTheDocument();

            await open();
            expect(within(popover()).getByRole('group', {name: 'Message priority'})).toBeInTheDocument();
            expect(within(popover()).getByRole('switch', {name: 'Request acknowledgement'})).toBeInTheDocument();
            expect(within(popover()).getByRole('switch', {name: 'Burn on read'})).toBeInTheDocument();
            expect(within(popover()).getByRole('switch', {name: 'Send as a spoiler'})).toBeInTheDocument();
        });

        test('what is set shows as chips with its state; a chip\'s cross removes it', async () => {
            render(options);
            await open();
            await userEvent.click(within(popover()).getByRole('button', {name: 'Urgent'}));
            await userEvent.click(within(popover()).getByRole('switch', {name: 'Request acknowledgement'}));
            await userEvent.click(within(popover()).getByRole('switch', {name: 'Send as a spoiler'}));
            await userEvent.click(within(popover()).getByRole('button', {name: 'Done'}));

            expect(screen.getByRole('button', {name: 'Urgent · acknowledgement'})).toBeInTheDocument();
            expect(screen.getByRole('button', {name: 'Spoiler'})).toBeInTheDocument();
            expect(screen.getByRole('button', {name: 'Message options'})).toHaveClass('am-on');

            await userEvent.click(screen.getByRole('button', {name: 'Remove priority'}));
            expect(screen.queryByRole('button', {name: /Urgent/})).not.toBeInTheDocument();
            await userEvent.click(screen.getByRole('button', {name: 'Don\'t send as a spoiler'}));
            expect(screen.queryByRole('button', {name: 'Spoiler'})).not.toBeInTheDocument();
            expect(screen.getByRole('button', {name: 'Message options'})).not.toHaveClass('am-on');
        });

        test('a chip opens the options on its own', async () => {
            render(options);
            await open();
            await userEvent.click(within(popover()).getByRole('switch', {name: 'Send as a spoiler'}));
            await userEvent.click(within(popover()).getByRole('button', {name: 'Done'}));

            await userEvent.click(screen.getByRole('button', {name: 'Spoiler'}));
            expect(within(popover()).getByRole('switch', {name: 'Send as a spoiler'})).toHaveFocus();
        });

        test('burning on read: the default duration until one is picked, then the one picked, sent with the message', async () => {
            render(options);
            await open();
            await userEvent.click(within(popover()).getByRole('switch', {name: 'Burn on read'}));

            // The default (10 min) isn't a step: the slider shows it as a custom duration.
            expect(screen.getByRole('button', {name: 'Burns 10 min after reading'})).toBeInTheDocument();
            const slider = within(popover()).getByRole('slider', {name: 'Deleted after reading'});
            expect(slider).toHaveAttribute('aria-valuetext', 'Custom: 10 minutes');

            // Over the administrator's maximum (4 h), no step: 1 day isn't offered.
            expect(slider).toHaveAttribute('max', '5');
            fireEvent.change(slider, {target: {value: '1'}});
            expect(screen.getByRole('button', {name: 'Burns 15 min after reading'})).toBeInTheDocument();
            fireEvent.change(slider, {target: {value: '4'}});
            expect(screen.getByRole('button', {name: 'Burns 4 h after reading'})).toBeInTheDocument();
            await userEvent.click(within(popover()).getByRole('button', {name: 'Done'}));

            await userEvent.type(screen.getByRole('textbox', {name: 'Write to General'}), 'Gone soon{Enter}');
            await waitFor(() => expect(onSubmit).toHaveBeenCalled());
            expect(sent()).toEqual(expect.objectContaining({message: 'Gone soon', type: 'burn_on_read', props: expect.objectContaining({read_duration: 4 * 60 * 60 * 1000})}));
        });

        test('without a duration picked, the message carries none: the server applies its default', async () => {
            render(options);
            await open();
            await userEvent.click(within(popover()).getByRole('switch', {name: 'Burn on read'}));
            await userEvent.click(within(popover()).getByRole('button', {name: 'Done'}));
            await userEvent.type(screen.getByRole('textbox', {name: 'Write to General'}), 'Default{Enter}');

            await waitFor(() => expect(onSubmit).toHaveBeenCalled());
            expect(sent().type).toBe('burn_on_read');
            expect(sent().props?.read_duration).toBeUndefined();
        });

        test('turning burning off drops the duration picked', async () => {
            render(options);
            await open();
            await userEvent.click(within(popover()).getByRole('switch', {name: 'Burn on read'}));
            fireEvent.change(within(popover()).getByRole('slider', {name: 'Deleted after reading'}), {target: {value: '2'}});
            await userEvent.click(within(popover()).getByRole('button', {name: 'Done'}));

            await userEvent.click(screen.getByRole('button', {name: 'Turn off burn on read'}));
            expect(screen.queryByRole('button', {name: /Burns/})).not.toBeInTheDocument();
            await userEvent.type(screen.getByRole('textbox', {name: 'Write to General'}), 'Plain{Enter}');

            await waitFor(() => expect(onSubmit).toHaveBeenCalled());
            expect(sent().type).toBeUndefined();
            expect(sent().props?.read_duration).toBeUndefined();
        });

        test('no burning where the channel doesn\'t allow it', async () => {
            mockBurnAllowed = false;
            render(options);
            await open();
            expect(within(popover()).queryByRole('switch', {name: 'Burn on read'})).not.toBeInTheDocument();
            expect(within(popover()).getByRole('switch', {name: 'Send as a spoiler'})).toBeInTheDocument();
        });
    });
});
