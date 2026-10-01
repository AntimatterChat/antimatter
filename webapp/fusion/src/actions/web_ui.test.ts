// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {Client4} from 'mattermost-redux/client';

import mockStore from 'tests/test_store';
import {
    CURRENT_WEB_UI,
    FUSION_ANNOUNCEMENT_PREFERENCE,
    WEB_UI_PREFERENCE,
    WebUIs,
    followPreferredWebUI,
    loadWebUI,
    takeRequestedWebUI,
} from 'utils/web_ui';

import {followWebUIPreference, switchWebUI, webUIPreferences} from './web_ui';

jest.mock('mattermost-redux/actions/preferences', () => ({
    savePreferences: jest.fn(() => ({type: 'MOCK_SAVE_PREFERENCES'})),
}));

jest.mock('utils/web_ui', () => ({
    ...jest.requireActual('utils/web_ui'),
    followPreferredWebUI: jest.fn(),
    loadWebUI: jest.fn(),
    takeRequestedWebUI: jest.fn(() => null),
}));

const OTHER_WEB_UI = CURRENT_WEB_UI === WebUIs.CLASSIC ? WebUIs.FUSION : WebUIs.CLASSIC;
const userId = 'user1';

function stateWith({allowed = true, preferred, announcement}: {allowed?: boolean; preferred?: string; announcement?: string}) {
    const myPreferences: Record<string, {category: string; name: string; value: string; user_id: string}> = {};
    if (preferred) {
        myPreferences[`${WEB_UI_PREFERENCE.CATEGORY}--${WEB_UI_PREFERENCE.NAME}`] = {category: WEB_UI_PREFERENCE.CATEGORY, name: WEB_UI_PREFERENCE.NAME, value: preferred, user_id: userId};
    }
    if (announcement) {
        myPreferences[`${FUSION_ANNOUNCEMENT_PREFERENCE.CATEGORY}--${FUSION_ANNOUNCEMENT_PREFERENCE.NAME}`] = {category: FUSION_ANNOUNCEMENT_PREFERENCE.CATEGORY, name: FUSION_ANNOUNCEMENT_PREFERENCE.NAME, value: announcement, user_id: userId};
    }
    return {
        entities: {
            general: {config: {AllowUserWebUISelection: String(allowed)}},
            users: {currentUserId: userId},
            preferences: {myPreferences},
        },
    };
}

describe('actions/web_ui', () => {
    beforeEach(() => {
        jest.clearAllMocks();
    });

    test('webUIPreferences saves the web UI and stops announcing Fusion', () => {
        expect(webUIPreferences(userId, WebUIs.FUSION)).toEqual([
            {user_id: userId, category: 'display_settings', name: 'web_ui', value: 'fusion'},
            {user_id: userId, category: 'antimatter_announcements', name: 'fusion_web_ui', value: 'dismissed'},
        ]);
    });

    describe('switchWebUI', () => {
        test('saves the web UI, then loads the page in it', async () => {
            const save = jest.spyOn(Client4, 'savePreferences').mockResolvedValue({status: 'OK'});
            const store = mockStore(stateWith({}));

            await store.dispatch(switchWebUI(OTHER_WEB_UI));

            expect(save).toHaveBeenCalledWith(userId, webUIPreferences(userId, OTHER_WEB_UI));
            expect(loadWebUI).toHaveBeenCalledWith(OTHER_WEB_UI);
        });

        test('loads the page in the web UI even when saving fails', async () => {
            jest.spyOn(Client4, 'savePreferences').mockRejectedValue(new Error('offline'));
            const store = mockStore(stateWith({}));

            await store.dispatch(switchWebUI(OTHER_WEB_UI));

            expect(loadWebUI).toHaveBeenCalledWith(OTHER_WEB_UI);
        });
    });

    describe('followWebUIPreference', () => {
        test('follows the preferred web UI', () => {
            const store = mockStore(stateWith({preferred: OTHER_WEB_UI}));
            store.dispatch(followWebUIPreference());
            expect(followPreferredWebUI).toHaveBeenCalledWith(OTHER_WEB_UI);
            expect(savePreferences).not.toHaveBeenCalled();
        });

        test('does nothing without a valid preference', () => {
            mockStore(stateWith({})).dispatch(followWebUIPreference());
            mockStore(stateWith({preferred: 'modern'})).dispatch(followWebUIPreference());
            expect(followPreferredWebUI).not.toHaveBeenCalled();
        });

        test('does nothing when users may not pick their web UI', () => {
            mockStore(stateWith({allowed: false, preferred: OTHER_WEB_UI})).dispatch(followWebUIPreference());
            expect(followPreferredWebUI).not.toHaveBeenCalled();
            expect(savePreferences).not.toHaveBeenCalled();
        });

        test('saves the web UI requested with ?webui= instead', () => {
            (takeRequestedWebUI as jest.Mock).mockReturnValueOnce(CURRENT_WEB_UI);
            mockStore(stateWith({preferred: OTHER_WEB_UI})).dispatch(followWebUIPreference());
            expect(savePreferences).toHaveBeenCalledWith(userId, webUIPreferences(userId, CURRENT_WEB_UI));
            expect(followPreferredWebUI).not.toHaveBeenCalled();
        });

        test('does not save the requested web UI again', () => {
            (takeRequestedWebUI as jest.Mock).mockReturnValueOnce(CURRENT_WEB_UI);
            mockStore(stateWith({preferred: CURRENT_WEB_UI, announcement: 'dismissed'})).dispatch(followWebUIPreference());
            expect(savePreferences).not.toHaveBeenCalled();
            expect(followPreferredWebUI).not.toHaveBeenCalled();
        });
    });
});
