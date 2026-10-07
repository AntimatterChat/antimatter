// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import StatusEmoji from './index';

jest.mock('components/emoji/render_emoji', () => ({
    __esModule: true,
    default: ({emojiName}: {emojiName: string}) => <span data-testid='emoji'>{emojiName}</span>,
}));

describe('fusion/components/StatusEmoji', () => {
    const render = (customStatus: object | undefined, enabled = 'true') => renderWithContext(
        <StatusEmoji userId='alice'/>,
        {
            entities: {
                general: {config: {EnableCustomUserStatuses: enabled}},
                users: {currentUserId: 'me', profiles: {alice: TestHelper.getUserMock({id: 'alice', props: customStatus ? {customStatus: JSON.stringify(customStatus)} : {}})}},
            },
        } as never,
    );

    test('shows the emoji of a custom status, with its text as tooltip', () => {
        render({emoji: 'palm_tree', text: 'On holiday', duration: 'date_and_time', expires_at: new Date(Date.now() + 864e5).toISOString()});

        expect(screen.getByTestId('emoji')).toHaveTextContent('palm_tree');
        expect(screen.getByLabelText('On holiday')).toHaveAttribute('data-am-tip', 'On holiday');
    });

    test('shows nothing for an expired status, no status, or when statuses are off', () => {
        render({emoji: 'palm_tree', text: 'Over', duration: 'date_and_time', expires_at: new Date(Date.now() - 864e5).toISOString()});
        render(undefined);
        render({emoji: 'palm_tree', text: 'Off', duration: ''}, 'false');

        expect(screen.queryByTestId('emoji')).not.toBeInTheDocument();
    });
});
