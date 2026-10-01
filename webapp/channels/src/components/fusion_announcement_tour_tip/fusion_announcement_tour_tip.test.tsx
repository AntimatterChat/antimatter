// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {getPreferenceKey} from 'mattermost-redux/utils/preference_utils';

import {switchWebUI} from 'actions/web_ui';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {FUSION_ANNOUNCEMENT_PREFERENCE, WEB_UI_PREFERENCE} from 'utils/web_ui';

import FusionAnnouncementTourTip from './fusion_announcement_tour_tip';

jest.mock('actions/web_ui', () => ({
    switchWebUI: jest.fn(() => ({type: 'MOCK_SWITCH_WEB_UI'})),
}));

jest.mock('mattermost-redux/actions/preferences', () => ({
    ...jest.requireActual('mattermost-redux/actions/preferences'),
    savePreferences: jest.fn(() => ({type: 'MOCK_SAVE_PREFERENCES'})),
}));

const userId = 'user1';

function stateWith({allowed = true, webUI = 'classic', announcement = ''}: {allowed?: boolean; webUI?: string; announcement?: string}) {
    const myPreferences: Record<string, {category: string; name: string; value: string; user_id: string}> = {};
    if (webUI) {
        myPreferences[getPreferenceKey(WEB_UI_PREFERENCE.CATEGORY, WEB_UI_PREFERENCE.NAME)] = {
            category: WEB_UI_PREFERENCE.CATEGORY,
            name: WEB_UI_PREFERENCE.NAME,
            value: webUI,
            user_id: userId,
        };
    }
    if (announcement) {
        myPreferences[getPreferenceKey(FUSION_ANNOUNCEMENT_PREFERENCE.CATEGORY, FUSION_ANNOUNCEMENT_PREFERENCE.NAME)] = {
            category: FUSION_ANNOUNCEMENT_PREFERENCE.CATEGORY,
            name: FUSION_ANNOUNCEMENT_PREFERENCE.NAME,
            value: announcement,
            user_id: userId,
        };
    }
    return {
        entities: {
            general: {config: {AllowUserWebUISelection: String(allowed)}},
            users: {currentUserId: userId},
            preferences: {myPreferences},
        },
    };
}

describe('components/fusion_announcement_tour_tip', () => {
    beforeEach(() => {
        jest.clearAllMocks();
    });

    test('announces Fusion to users kept on the classic web UI', () => {
        renderWithContext(<FusionAnnouncementTourTip/>, stateWith({}));
        expect(screen.getByText('A new interface is available')).toBeInTheDocument();
        expect(screen.getByText('Settings > Display > Web interface')).toBeInTheDocument();
    });

    test.each([
        ['users may not pick their web UI', {allowed: false}],
        ['the user has no web UI preference', {webUI: ''}],
        ['the user prefers Fusion', {webUI: 'fusion'}],
        ['the user dismissed the announcement', {announcement: 'dismissed'}],
    ])('announces nothing when %s', (_, state) => {
        const {container} = renderWithContext(<FusionAnnouncementTourTip/>, stateWith(state));
        expect(container).toBeEmptyDOMElement();
        expect(screen.queryByText('A new interface is available')).not.toBeInTheDocument();
    });

    test('switches to Fusion', async () => {
        renderWithContext(<FusionAnnouncementTourTip/>, stateWith({}));
        await userEvent.click(screen.getByText('Try Fusion'));
        expect(switchWebUI).toHaveBeenCalledWith('fusion');
    });

    test('stops announcing Fusion when the user chooses not now', async () => {
        renderWithContext(<FusionAnnouncementTourTip/>, stateWith({}));
        await userEvent.click(screen.getByText('Not now'));
        expect(savePreferences).toHaveBeenCalledWith(userId, [{
            user_id: userId,
            category: 'antimatter_announcements',
            name: 'fusion_web_ui',
            value: 'dismissed',
        }]);
        expect(switchWebUI).not.toHaveBeenCalled();
    });

    test('closes to its pulsating dot, which opens it again', async () => {
        renderWithContext(<FusionAnnouncementTourTip/>, stateWith({}));
        await userEvent.click(screen.getByTestId('close_tutorial_tip'));
        await waitFor(() => expect(screen.queryByText('A new interface is available')).not.toBeInTheDocument());
        expect(savePreferences).not.toHaveBeenCalled();

        await userEvent.click(document.getElementById('tipButton')!);
        expect(await screen.findByText('A new interface is available')).toBeInTheDocument();
    });
});
