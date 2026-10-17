// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import SleepNote from './sleep_note';

describe('fusion/composer/SleepNote', () => {
    const me = TestHelper.getUserMock({id: 'me', username: 'me'});
    const dm = TestHelper.getChannelMock({id: 'me__bob', name: 'bob__me', type: 'D', teammate_id: 'bob'} as never);
    const bobIn = (zone: string) => TestHelper.getUserMock({
        id: 'bob',
        username: 'bob',
        first_name: 'Bob',
        timezone: {useAutomaticTimezone: 'false', automaticTimezone: '', manualTimezone: zone},
    });
    const render = (zone: string, onSendAt = jest.fn()) => {
        renderWithContext(
            <SleepNote
                channelId={dm.id}
                onSendAt={onSendAt}
            />,
            {
                entities: {
                    general: {config: {ScheduledPosts: 'true'}},
                    channels: {channels: {[dm.id]: dm}, myMembers: {}, channelsInTeam: {}},
                    users: {currentUserId: 'me', profiles: {me, bob: bobIn(zone)}, profilesInChannel: {}},
                },
            } as never,
        );
        return onSendAt;
    };

    beforeEach(() => {
        // 22:00 UTC: midnight in Berlin, 05:00 in Bangkok.
        jest.useFakeTimers({advanceTimers: true}).setSystemTime(new Date('2026-10-17T22:00:00Z'));
    });
    afterEach(() => {
        jest.useRealTimers();
    });

    test('offers to send at the start of the working day in their country, not at 07:00', async () => {
        const onSendAt = render('Europe/Berlin');
        expect(screen.getByRole('note')).toHaveTextContent(/for Bob — they may be asleep/);

        const later = screen.getByRole('button', {name: /^Send at .*08:00.* their time$/});
        expect(later).not.toHaveTextContent(/07:00/);
        await userEvent.click(later);
        expect(onSendAt).toHaveBeenCalledWith(Date.parse('2026-10-18T06:00:00Z'), {name: 'Bob', time: expect.stringMatching(/08:00/)});
    });

    test('sends at 08:30 their time when their country has no usual hour', async () => {
        const onSendAt = render('Asia/Bangkok');

        const later = screen.getByRole('button', {name: /^Send at .*08:30.* their time$/});
        await userEvent.click(later);
        expect(onSendAt).toHaveBeenCalledWith(Date.parse('2026-10-18T01:30:00Z'), {name: 'Bob', time: expect.stringMatching(/08:30/)});
    });

    test('stays away when they are awake', () => {
        render('America/New_York');
        expect(screen.queryByRole('note')).not.toBeInTheDocument();
    });
});
