// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {deletePreferences, savePreferences} from 'mattermost-redux/actions/preferences';
import {updateMe} from 'mattermost-redux/actions/users';

import {renderWithContext, screen, userEvent, within} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import CommunityPrivacyDialog from './community_privacy_dialog';

jest.mock('mattermost-redux/actions/preferences', () => ({
    savePreferences: jest.fn(() => ({type: 'MOCK_SAVE_PREFERENCES'})),
    deletePreferences: jest.fn(() => ({type: 'MOCK_DELETE_PREFERENCES'})),
}));
jest.mock('mattermost-redux/actions/users', () => ({
    ...jest.requireActual('mattermost-redux/actions/users'),
    updateMe: jest.fn(() => ({type: 'MOCK_UPDATE_ME'})),
}));

const me = 'me';
const pref = (category: string, name: string, value: string) => ({[`${category}--${name}`]: {user_id: me, category, name, value}});

function state({preferences = {}, props = {}, config = {}}: {preferences?: Record<string, unknown>; props?: Record<string, string>; config?: Record<string, string>} = {}) {
    return {
        entities: {
            general: {config},
            users: {currentUserId: me, profiles: {me: TestHelper.getUserMock({id: me, props})}},
            teams: {teams: {t1: TestHelper.getTeamMock({id: 't1', display_name: 'Lumen', delete_at: 0})}, myMembers: {t1: {team_id: 't1', user_id: me, roles: 'team_user'}}},
            channels: {channels: {}, myMembers: {}},
            preferences: {myPreferences: preferences},
        },
    };
}

// A community's own privacy: how you take its people's direct messages, and which profile details they see.
describe('fusion/privacy/CommunityPrivacyDialog', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const render = (options?: Parameters<typeof state>[0]) => renderWithContext(
        <CommunityPrivacyDialog
            teamId='t1'
            onClose={jest.fn()}
        />,
        state(options) as never,
    );

    test('direct messages follow your default until the community has its own choice', async () => {
        render({preferences: pref('dm_privacy', 'default', 'ask')});
        expect(screen.getByRole('dialog', {name: 'Privacy in Lumen'})).toBeInTheDocument();
        const select = screen.getByRole('combobox', {name: 'From people you don\'t talk with yet'});
        expect(select).toHaveValue('');
        expect(within(select).getByRole('option', {name: 'As my default (Ask me first)'})).toBeInTheDocument();

        await userEvent.selectOptions(select, 'refuse');
        expect(savePreferences).toHaveBeenCalledWith(me, [{user_id: me, category: 'dm_privacy', name: 't1', value: 'refuse'}]);
    });

    test('back to "as my default" forgets the community\'s choice', async () => {
        render({preferences: pref('dm_privacy', 't1', 'refuse')});
        const select = screen.getByRole('combobox', {name: 'From people you don\'t talk with yet'});
        expect(select).toHaveValue('refuse');
        await userEvent.selectOptions(select, '');
        expect(deletePreferences).toHaveBeenCalledWith(me, [{user_id: me, category: 'dm_privacy', name: 't1', value: ''}]);
    });

    test('the profile details start as your defaults; changing one gives the community its own choice', async () => {
        render({props: {profile_privacy: JSON.stringify({default: ['position']})}});
        expect(screen.getByText(/As your defaults until you change something here/)).toBeInTheDocument();
        expect(screen.getByRole('switch', {name: 'Position'})).toHaveAttribute('aria-checked', 'false');
        expect(screen.queryByRole('button', {name: 'Use my defaults'})).not.toBeInTheDocument();

        await userEvent.click(screen.getByRole('switch', {name: 'Full name'}));
        expect(updateMe).toHaveBeenCalledWith({props: {profile_privacy: JSON.stringify({default: ['position'], t1: ['full_name', 'position']})}});
    });

    test('a community that shows everything keeps an empty choice, and "Use my defaults" forgets it', async () => {
        render({props: {profile_privacy: JSON.stringify({default: ['position'], t1: []})}});
        expect(screen.getByRole('switch', {name: 'Position'})).toHaveAttribute('aria-checked', 'true');
        expect(screen.getByText(/Chosen for this community/)).toBeInTheDocument();

        await userEvent.click(screen.getByRole('button', {name: 'Use my defaults'}));
        expect(updateMe).toHaveBeenCalledWith({props: {profile_privacy: JSON.stringify({default: ['position']})}});
    });

    test('while the admin turned blocking off, only the profile details', () => {
        render({config: {EnableUserBlocking: 'false'}});
        expect(screen.queryByRole('combobox', {name: 'From people you don\'t talk with yet'})).not.toBeInTheDocument();
        expect(screen.getByRole('switch', {name: 'Nickname'})).toBeInTheDocument();
    });
});
