// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import {RowContent} from './autocomplete';

jest.mock('fusion/components/avatar', () => () => null);

// Antimatter: the @ suggestions show someone's nickname in the conversation's community first.
describe('fusion/composer/autocomplete: community profiles', () => {
    const sofia = TestHelper.getUserMock({id: 'sofia', username: 'sofia', first_name: 'Sofia', last_name: 'Reyes', nickname: 'Sof'});
    const state = (profiles: Record<string, unknown>) => ({
        entities: {
            general: {config: {}},
            teams: {currentTeamId: 't1', teams: {t1: TestHelper.getTeamMock({id: 't1'})}, communityProfiles: {t1: profiles}},
            channels: {currentChannelId: 'c1', channels: {c1: TestHelper.getChannelMock({id: 'c1', team_id: 't1', type: 'O'})}},
        },
    }) as never;

    test('their nickname in the community, then their full name', () => {
        renderWithContext(
            <RowContent
                kind='mention'
                row={{term: '@so', item: sofia}}
            />,
            state({sofia: {team_id: 't1', user_id: 'sofia', nickname: 'Captain', update_at: 1}}),
        );
        expect(screen.getByText('Captain · Sofia Reyes')).toBeInTheDocument();
    });

    test('their full name, else their global nickname, without a profile there', () => {
        renderWithContext(
            <RowContent
                kind='mention'
                row={{term: '@so', item: {...sofia, first_name: '', last_name: ''}}}
            />,
            state({}),
        );
        expect(screen.getByText('Sof')).toBeInTheDocument();
    });
});
