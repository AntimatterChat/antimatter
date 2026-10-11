// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';

import WalltimeText from './walltime_text';

describe('WalltimeText', () => {
    // 3pm in New York on Friday October 9 2026, read in Paris.
    const start = Date.UTC(2026, 9, 9, 19, 0);
    const state = (zone: string, walltime = 'true') => ({
        entities: {
            users: {currentUserId: 'me', profiles: {me: {id: 'me', timezone: {useAutomaticTimezone: 'false', manualTimezone: zone, automaticTimezone: ''}}}},
            preferences: {myPreferences: {'display_settings--walltime': {category: 'display_settings', name: 'walltime', user_id: 'me', value: walltime}}},
        },
    });
    const render = (props: Partial<React.ComponentProps<typeof WalltimeText>> = {}, zone = 'Europe/Paris', walltime = 'true') => renderWithContext(
        <p>
            {'Sync at '}
            <WalltimeText
                start={start}
                zone='America/New_York'
                explicit={false}
                {...props}
            >
                {'3pm'}
            </WalltimeText>
        </p>,
        state(zone, walltime),
    );

    test('shows the reader\'s time in its tooltip', () => {
        const {container} = render();
        const time = container.querySelector('time')!;
        expect(time).toHaveAttribute('dateTime', '2026-10-09T19:00:00.000Z');
        expect(time.dataset.amTip).toMatch(/9:00\sPM GMT\+2.* your time · .*3:00\sPM EDT.* as written/);
        expect(time).toHaveTextContent('3pm');
        expect(time.querySelector('svg use')).toHaveAttribute('href', '#am-i-clock');
    });

    test('leaves the time alone for a reader in the author\'s zone', () => {
        const {container} = render({}, 'America/Toronto');
        expect(container.querySelector('time')).toBeNull();
        expect(screen.getByText('Sync at 3pm')).toBeInTheDocument();
    });

    test('highlights a time given with its zone, even in the reader\'s', () => {
        const {container} = render({explicit: true}, 'America/New_York');
        expect(container.querySelector('time')).not.toBeNull();
    });

    test('can be turned off', () => {
        const {container} = render({}, 'Europe/Paris', 'false');
        expect(container.querySelector('time')).toBeNull();
    });
});
