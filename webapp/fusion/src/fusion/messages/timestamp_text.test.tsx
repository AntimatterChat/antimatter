// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {act, renderWithContext, screen} from 'tests/react_testing_utils';
import {formatText} from 'utils/text_formatting';

import TimestampText, {RELATIVE_TICK_MS} from './timestamp_text';

// The tests' react-intl answers in English whatever the provider says (tests/react-intl_mock): the language is set here.
let mockLocale = 'en';
jest.mock('react-intl', () => {
    const actual = jest.requireActual('react-intl');
    return {
        ...actual,
        useIntl: () => actual.createIntl({locale: mockLocale, defaultLocale: 'en', messages: {}}),
    };
});

const emojiMap = {has: () => false, get: () => undefined} as any; // eslint-disable-line @typescript-eslint/no-explicit-any

// sec-fusion #12: <t:…> timestamps were formatted once, in the browser's language and time zone, past the reader's
// settings, and a relative one stayed as it was first drawn.
describe('fusion/messages/TimestampText', () => {
    const state = (timezone: Record<string, string>) => ({
        entities: {
            users: {currentUserId: 'me', profiles: {me: {id: 'me', username: 'me', timezone}}},
        },
    } as never);

    afterEach(() => {
        jest.useRealTimers();
        mockLocale = 'en';
    });

    test('formatText leaves the timestamp for the component to draw', () => {
        const html = formatText('Starts <t:0:t>', {}, emojiMap);
        expect(html).toContain('data-timestamp="0"');
        expect(html).toContain('data-timestamp-style="t"');
    });

    test('shows the time in the time zone of the reader\'s settings', () => {
        // 1 January 2026, 00:00 UTC: 09:00 in Tokyo.
        const at = Date.UTC(2026, 0, 1, 0, 0) / 1000;
        renderWithContext(
            <TimestampText
                seconds={at}
                format='t'
            />,
            state({useAutomaticTimezone: 'false', manualTimezone: 'Asia/Tokyo', automaticTimezone: ''}),
        );
        expect(screen.getByText(/^9:00/)).toBeInTheDocument();
    });

    test('shows the time in the reader\'s language', () => {
        mockLocale = 'fr';
        const at = Date.UTC(2026, 0, 15, 12, 0) / 1000;
        renderWithContext(
            <TimestampText
                seconds={at}
                format='D'
            />,
            state({useAutomaticTimezone: 'false', manualTimezone: 'Europe/Paris', automaticTimezone: ''}),
        );
        expect(screen.getByText('15 janvier 2026')).toBeInTheDocument();
    });

    test('keeps a relative time up to date', () => {
        jest.useFakeTimers();
        jest.setSystemTime(Date.UTC(2026, 0, 1, 12, 0));
        const at = Date.UTC(2026, 0, 1, 12, 5) / 1000;
        renderWithContext(
            <TimestampText
                seconds={at}
                format='R'
            />,
            state({useAutomaticTimezone: 'true', manualTimezone: '', automaticTimezone: 'UTC'}),
        );
        expect(screen.getByText('in 5 minutes')).toBeInTheDocument();

        act(() => {
            jest.advanceTimersByTime(60 * 60 * 1000);
        });
        expect(screen.getByText('55 minutes ago')).toBeInTheDocument();
        expect(RELATIVE_TICK_MS).toBeLessThanOrEqual(60000);
    });

    test('a time zone the browser doesn\'t know falls back to its own', () => {
        renderWithContext(
            <TimestampText
                seconds={0}
                format='D'
            />,
            state({useAutomaticTimezone: 'false', manualTimezone: 'Not/AZone', automaticTimezone: ''}),
        );
        expect(screen.getByText(/19(69|70)/)).toBeInTheDocument();
    });
});
