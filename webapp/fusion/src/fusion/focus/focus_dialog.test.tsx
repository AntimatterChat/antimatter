// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Client4} from 'mattermost-redux/client';

import {fireEvent, renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import FocusDialog, {tomorrowAt9} from './focus_dialog';

const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));

// 14:10 in Paris.
const NOW = new Date('2026-10-21T12:10:00Z').getTime();

describe('fusion/focus/FocusDialog', () => {
    const onClose = jest.fn();
    const render = () => renderWithContext(
        <FocusDialog onClose={onClose}/>,
        {
            entities: {
                users: {currentUserId: 'me', profiles: {me: {id: 'me', username: 'me', timezone: {useAutomaticTimezone: 'false', manualTimezone: 'Europe/Paris', automaticTimezone: ''}}}},
                general: {config: {ExperimentalTimezone: 'true'}},
            },
        } as never,
    );

    beforeEach(() => {
        jest.useFakeTimers({advanceTimers: true}).setSystemTime(NOW);
        jest.clearAllMocks();
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
        Client4.startFocus = jest.fn().mockResolvedValue({active: true});
    });
    afterEach(() => {
        jest.useRealTimers();
    });

    test('offers the durations of the decided design, an hour picked', () => {
        render();
        const chips = screen.getAllByRole('radio').map((chip) => chip.textContent);
        expect(chips).toEqual(['15 min', '30 min', '1 h', '2 h', '3 h', '4 h', '5 h', '6 h', 'Until tomorrow', 'Custom']);
        expect(screen.getByRole('radio', {name: '1 h'})).toHaveAttribute('aria-checked', 'true');
        expect(screen.getByText(/^Until /)).toBeInTheDocument();
    });

    test('starts for the duration picked, with the reason', async () => {
        render();
        await userEvent.click(screen.getByRole('radio', {name: '30 min'}));
        expect(screen.getByRole('radio', {name: '30 min'})).toHaveAttribute('aria-checked', 'true');
        await userEvent.type(screen.getByRole('textbox'), '  Working on the new release ');
        await userEvent.click(screen.getByRole('button', {name: 'Start'}));

        expect(Client4.startFocus).toHaveBeenCalledWith({duration: '30m', reason: 'Working on the new release'});
        await waitFor(() => expect(onClose).toHaveBeenCalled());
        expect(mockToast).toHaveBeenCalledWith(expect.stringMatching(/^Focusing until /));
    });

    test('until tomorrow is 09:00 tomorrow in your timezone, worked out by the server', async () => {
        render();
        await userEvent.click(screen.getByRole('radio', {name: 'Until tomorrow'}));
        expect(screen.getByText(/^Until .*9:00/)).toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: 'Start'}));
        expect(Client4.startFocus).toHaveBeenCalledWith({duration: 'tomorrow', reason: ''});

        expect(new Date(tomorrowAt9('Europe/Paris', NOW)).toISOString()).toBe('2026-10-22T07:00:00.000Z');
        expect(new Date(tomorrowAt9('Europe/Paris', new Date('2026-10-21T22:30:00Z').getTime())).toISOString()).toBe('2026-10-23T07:00:00.000Z');
    });

    test('a custom end takes a date and time, within a minute and a week', async () => {
        render();
        expect(screen.queryByLabelText('Focus until')).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('radio', {name: 'Custom'}));
        const field = screen.getByLabelText('Focus until');

        // In the past (14:00 in Paris is before now), then more than a week away.
        fireEvent.change(field, {target: {value: '2026-10-21T14:00'}});
        expect(field).toHaveAttribute('aria-invalid', 'true');
        expect(screen.getByText('Choose a time from a minute to a week from now.')).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Start'})).toBeDisabled();
        fireEvent.change(field, {target: {value: '2026-11-02T10:00'}});
        expect(screen.getByRole('button', {name: 'Start'})).toBeDisabled();

        // 17:45 in Paris.
        fireEvent.change(field, {target: {value: '2026-10-21T17:45'}});
        expect(field).toHaveAttribute('aria-invalid', 'false');
        expect(screen.getByRole('button', {name: 'Start'})).toBeEnabled();
        await userEvent.click(screen.getByRole('button', {name: 'Start'}));
        expect(Client4.startFocus).toHaveBeenCalledWith({end_at: new Date('2026-10-21T15:45:00Z').getTime(), reason: ''});
    });

    test('the reason is a custom status: optional, 100 characters, with an example', () => {
        render();
        const reason = screen.getByRole('textbox');
        expect(reason).toHaveAttribute('maxLength', '100');
        expect(reason).toHaveAttribute('placeholder', 'Working on the new release');
        expect(screen.getByText('Reason (optional)')).toBeInTheDocument();
    });

    test('tells when the server refuses, and stays open', async () => {
        Client4.startFocus = jest.fn().mockRejectedValue({message: 'Focus mode needs user statuses, which are turned off on this server.'});
        render();
        await userEvent.click(screen.getByRole('button', {name: 'Start'}));
        expect(await screen.findByRole('alert')).toHaveTextContent('Focus mode needs user statuses');
        expect(onClose).not.toHaveBeenCalled();
    });
});
