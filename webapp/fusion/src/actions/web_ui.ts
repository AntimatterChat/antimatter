// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {PreferenceType} from '@mattermost/types/preferences';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {Client4} from 'mattermost-redux/client';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {
    FUSION_ANNOUNCEMENT_PREFERENCE,
    WEB_UI_PREFERENCE,
    followPreferredWebUI,
    isWebUI,
    loadWebUI,
    takeRequestedWebUI,
} from 'utils/web_ui';
import type {WebUI} from 'utils/web_ui';

import type {ActionFuncAsync, GlobalState, ThunkActionFunc} from 'types/store';

// webUIPreferences are the preferences saved when the user picks a web UI. Picking one also means
// they know about the Fusion UI, so it isn't announced to them anymore.
export function webUIPreferences(userId: string, webUI: WebUI): PreferenceType[] {
    return [
        {user_id: userId, category: WEB_UI_PREFERENCE.CATEGORY, name: WEB_UI_PREFERENCE.NAME, value: webUI},
        {
            user_id: userId,
            category: FUSION_ANNOUNCEMENT_PREFERENCE.CATEGORY,
            name: FUSION_ANNOUNCEMENT_PREFERENCE.NAME,
            value: FUSION_ANNOUNCEMENT_PREFERENCE.DISMISSED,
        },
    ];
}

function isUserWebUISelectionAllowed(state: GlobalState) {
    return getConfig(state).AllowUserWebUISelection === 'true';
}

// switchWebUI saves the web UI picked by the user, then loads the page in it.
export function switchWebUI(webUI: WebUI): ActionFuncAsync<boolean> {
    return async (dispatch, getState) => {
        const userId = getCurrentUserId(getState());
        if (userId) {
            try {
                await Client4.savePreferences(userId, webUIPreferences(userId, webUI));
            } catch {
                // The web UI it loads saves it again.
            }
        }
        loadWebUI(webUI);
        return {data: true};
    };
}

// followWebUIPreference runs once the user is signed in. When the page was opened with ?webui=, it
// saves that web UI as the user's preference. Otherwise, it makes this browser follow the user's
// preference, reloading the page if it was served another web UI.
export function followWebUIPreference(): ThunkActionFunc<void, GlobalState> {
    return (dispatch, getState) => {
        const state = getState();
        const userId = getCurrentUserId(state);
        if (!userId || !isUserWebUISelectionAllowed(state)) {
            return;
        }

        const preferred = getPreference(state, WEB_UI_PREFERENCE.CATEGORY, WEB_UI_PREFERENCE.NAME, '');
        const requested = takeRequestedWebUI();
        if (requested) {
            const announced = getPreference(state, FUSION_ANNOUNCEMENT_PREFERENCE.CATEGORY, FUSION_ANNOUNCEMENT_PREFERENCE.NAME, '');
            if (preferred !== requested || announced !== FUSION_ANNOUNCEMENT_PREFERENCE.DISMISSED) {
                dispatch(savePreferences(userId, webUIPreferences(userId, requested)));
            }
            return;
        }

        if (isWebUI(preferred)) {
            followPreferredWebUI(preferred);
        }
    };
}
