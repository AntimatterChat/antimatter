// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {UserProfile} from '@mattermost/types/users';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import {TeamCardHost, TeamTag} from './team_card';

// A team tag in a member row (a label inside the row's button): a click on it opens the team's card, as the tag does in
// messages and profile cards, not the person's profile card.
describe('fusion/teams/TeamTag in a member row', () => {
    beforeEach(() => {
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const user = {id: 'u1', username: 'ann', first_name: 'Ann', roles: 'system_user', props: {team_badge: JSON.stringify({t1: 'g1'})}} as unknown as UserProfile;
    const design = {id: 'g1', name: 'design', display_name: 'Design', tag: 'DES', source: 'custom', is_team: true, team_id: 't1', delete_at: 0, member_ids: ['u1'], description: ''};
    const render = (onRow: jest.Mock) => renderWithContext(
        <>
            <button
                type='button'
                onClick={onRow}
            >
                <span>{'Ann'}</span>
                <TeamTag
                    user={user}
                    label={true}
                />
            </button>
            <TeamCardHost/>
        </>,
        {
            entities: {
                users: {currentUserId: 'u1', profiles: {u1: user}},
                teams: {currentTeamId: 't1', teams: {t1: {id: 't1', name: 'one', display_name: 'One', delete_at: 0}}},
                groups: {groups: {g1: design}},
            },
        } as never,
    );

    test('a click on the tag opens the team\'s card, not the row\'s profile', async () => {
        const onRow = jest.fn();
        render(onRow);
        await userEvent.click(screen.getByText('DES'));
        expect(onRow).not.toHaveBeenCalled();
        expect(screen.getByRole('dialog', {name: 'Design'})).toBeInTheDocument();
    });

    test('a click elsewhere on the row still opens the profile', async () => {
        const onRow = jest.fn();
        render(onRow);
        await userEvent.click(screen.getByText('Ann'));
        expect(onRow).toHaveBeenCalled();
        expect(screen.queryByRole('dialog', {name: 'Design'})).not.toBeInTheDocument();
    });

    test('the tag stays a label: no button inside the row\'s button', () => {
        render(jest.fn());
        expect(screen.getByRole('button').querySelector('button')).toBeNull();
    });
});
