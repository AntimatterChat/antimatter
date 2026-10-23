// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {deletePreferences, savePreferences} from 'mattermost-redux/actions/preferences';

import {renderWithContext, screen, userEvent, within} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import CommunityNotificationsDialog from './community_notifications_dialog';
import {getCommunityNotifyProps, parseCommunityNotifyProps, withCommunityNotifyProps} from './community_notify';

jest.mock('mattermost-redux/actions/preferences', () => ({
    savePreferences: jest.fn(() => ({type: 'MOCK_SAVE_PREFERENCES'})),
    deletePreferences: jest.fn(() => ({type: 'MOCK_DELETE_PREFERENCES'})),
}));
jest.mock('utils/notification_sounds', () => ({
    ...jest.requireActual('utils/notification_sounds'),
    tryNotificationSound: jest.fn(),
}));

const me = 'me';
const notifyPref = (value: Record<string, string>) => ({'community_notify--t1': {user_id: me, category: 'community_notify', name: 't1', value: JSON.stringify(value)}});

function state(preferences: Record<string, unknown> = {}, config: Record<string, string> = {}) {
    return {
        entities: {
            general: {config: {SendPushNotifications: 'true', SendEmailNotifications: 'true', ...config}},
            users: {currentUserId: me, profiles: {me: TestHelper.getUserMock({id: me, notify_props: {desktop: 'all', push: 'mention', email: 'true', channel: 'true', desktop_sound: 'true'} as never})}},
            teams: {teams: {t1: TestHelper.getTeamMock({id: 't1', display_name: 'Lumen', delete_at: 0})}, myMembers: {}},
            channels: {channels: {}, myMembers: {}},
            preferences: {myPreferences: preferences},
        },
    };
}

describe('fusion/modals/CommunityNotificationsDialog', () => {
    beforeEach(() => {
        jest.clearAllMocks();
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const render = (preferences?: Record<string, unknown>, config?: Record<string, string>) => renderWithContext(
        <CommunityNotificationsDialog
            teamId='t1'
            onClose={jest.fn()}
        />,
        state(preferences, config) as never,
    );

    test('each setting follows yours, named, until the community sets its own', async () => {
        render();
        expect(screen.getByRole('dialog', {name: 'Notifications in Lumen'})).toBeInTheDocument();
        const push = screen.getByRole('combobox', {name: 'Push notifications about'});
        expect(push).toHaveValue('');
        expect(within(push).getByRole('option', {name: 'As in my settings (Mentions)'})).toBeInTheDocument();
        expect(within(screen.getByRole('combobox', {name: 'Notify me about'})).getByRole('option', {name: 'As in my settings (All)'})).toBeInTheDocument();

        await userEvent.selectOptions(push, 'none');
        expect(savePreferences).toHaveBeenCalledWith(me, [{user_id: me, category: 'community_notify', name: 't1', value: '{"push":"none"}'}]);
        expect(screen.queryByRole('button', {name: 'Use my settings everywhere'})).not.toBeInTheDocument();
    });

    test('setting one keeps the others; back to "as in my settings" drops it', async () => {
        render(notifyPref({push: 'none', desktop: 'mention'}));
        expect(screen.getByRole('combobox', {name: 'Push notifications about'})).toHaveValue('none');

        await userEvent.selectOptions(screen.getByRole('combobox', {name: 'Channel-wide mentions'}), 'false');
        expect(savePreferences).toHaveBeenCalledWith(me, [{user_id: me, category: 'community_notify', name: 't1', value: '{"push":"none","desktop":"mention","channel":"false"}'}]);

        await userEvent.selectOptions(screen.getByRole('combobox', {name: 'Push notifications about'}), '');
        expect(savePreferences).toHaveBeenLastCalledWith(me, [{user_id: me, category: 'community_notify', name: 't1', value: '{"desktop":"mention"}'}]);
    });

    test('the last setting gone, or "Use my settings everywhere", forgets the community\'s', async () => {
        render(notifyPref({push: 'none'}));
        await userEvent.selectOptions(screen.getByRole('combobox', {name: 'Push notifications about'}), '');
        expect(deletePreferences).toHaveBeenCalledWith(me, [expect.objectContaining({category: 'community_notify', name: 't1'})]);

        jest.clearAllMocks();
        await userEvent.click(screen.getByRole('button', {name: 'Use my settings everywhere'}));
        expect(deletePreferences).toHaveBeenCalledWith(me, [expect.objectContaining({category: 'community_notify', name: 't1'})]);
    });

    test('mobile and email only when the server sends them', () => {
        render({}, {SendPushNotifications: 'false', SendEmailNotifications: 'false'});
        expect(screen.queryByRole('combobox', {name: 'Push notifications about'})).not.toBeInTheDocument();
        expect(screen.queryByRole('combobox', {name: 'Email notifications'})).not.toBeInTheDocument();
        expect(screen.getByRole('combobox', {name: 'Notify me about'})).toBeInTheDocument();
    });
});

describe('fusion/modals/community_notify', () => {
    test('reads only the settings a community can set', () => {
        expect(parseCommunityNotifyProps('{"push":"none","mention_keys":"x","desktop":""}')).toEqual({push: 'none'});
        expect(parseCommunityNotifyProps('nope')).toEqual({});
        expect(parseCommunityNotifyProps('')).toEqual({});
    });

    test('a community\'s settings over the person\'s own, for that community only', () => {
        const s = state(notifyPref({desktop: 'none'})) as never;
        expect(getCommunityNotifyProps(s, 't1')).toEqual({desktop: 'none'});
        expect(getCommunityNotifyProps(s, 't2')).toEqual({});
        expect(getCommunityNotifyProps(s, '')).toEqual({});

        const user = TestHelper.getUserMock({notify_props: {desktop: 'all', push: 'mention'} as never});
        expect(withCommunityNotifyProps(user, {desktop: 'none'}).notify_props).toEqual(expect.objectContaining({desktop: 'none', push: 'mention'}));
        expect(withCommunityNotifyProps(user, {})).toBe(user);
        expect(user.notify_props.desktop).toBe('all');
    });
});
