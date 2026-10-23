// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {UserNotifyProps, UserProfile} from '@mattermost/types/users';

import {deletePreferences, savePreferences} from 'mattermost-redux/actions/preferences';
import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import type {ActionFuncAsync, GlobalState} from 'types/store';

// Notification settings per community: someone's notification settings can differ in one of their communities. They
// are a preference (community_notify, named after the community's id) whose value is a JSON object of the notify
// props it sets over the person's own. The server applies them to push and email notifications and mentions of the
// community's channels; the app applies them to desktop notifications and sounds. A channel's own notification
// preferences still win; direct and group messages follow the person's own settings.
export const COMMUNITY_NOTIFY_CATEGORY = 'community_notify';

export const COMMUNITY_NOTIFY_KEYS = [
    'desktop',
    'desktop_threads',
    'desktop_sound',
    'desktop_notification_sound',
    'push',
    'push_threads',
    'email',
    'channel',
] as const;

export type CommunityNotifyKey = typeof COMMUNITY_NOTIFY_KEYS[number];
export type CommunityNotifyProps = Partial<Record<CommunityNotifyKey, string>>;

const NONE: CommunityNotifyProps = {};

// The parsed settings, by their stored value, so that selectors give the same object while it doesn't change.
const parsed = new Map<string, CommunityNotifyProps>();

export function parseCommunityNotifyProps(raw: string): CommunityNotifyProps {
    if (!raw) {
        return NONE;
    }
    let props = parsed.get(raw);
    if (!props) {
        props = {};
        try {
            const value = JSON.parse(raw);
            if (value && typeof value === 'object' && !Array.isArray(value)) {
                for (const key of COMMUNITY_NOTIFY_KEYS) {
                    if (typeof value[key] === 'string' && value[key]) {
                        props[key] = value[key];
                    }
                }
            }
        } catch {
            // Settings that can't be read are none: the person's own apply.
        }
        parsed.set(raw, props);
    }
    return props;
}

// getCommunityNotifyProps gives the settings someone set for a community over their own ({} for none).
export function getCommunityNotifyProps(state: GlobalState, communityId: string): CommunityNotifyProps {
    if (!communityId) {
        return NONE;
    }
    return parseCommunityNotifyProps(getPreference(state, COMMUNITY_NOTIFY_CATEGORY, communityId, ''));
}

// withCommunityNotifyProps gives the user with a community's settings over their own: the user itself without any.
export function withCommunityNotifyProps<T extends Pick<UserProfile, 'notify_props'>>(user: T, props: CommunityNotifyProps): T {
    if (!user || !Object.keys(props).length) {
        return user;
    }
    return {...user, notify_props: {...user.notify_props, ...props} as UserNotifyProps};
}

// setCommunityNotifyProp sets one of a community's settings, or ('') lets it follow the person's own again; with
// none left, the preference goes.
export function setCommunityNotifyProp(communityId: string, key: CommunityNotifyKey, value: string): ActionFuncAsync {
    return async (dispatch, getState) => {
        const state = getState();
        const me = getCurrentUserId(state);
        const next: CommunityNotifyProps = {...getCommunityNotifyProps(state, communityId)};
        if (value) {
            next[key] = value;
        } else {
            delete next[key];
        }
        const pref = {user_id: me, category: COMMUNITY_NOTIFY_CATEGORY, name: communityId, value: JSON.stringify(next)};
        return Object.keys(next).length ? dispatch(savePreferences(me, [pref])) : dispatch(deletePreferences(me, [pref]));
    };
}

// resetCommunityNotifyProps lets all of a community's notifications follow the person's own settings again.
export function resetCommunityNotifyProps(communityId: string): ActionFuncAsync {
    return async (dispatch, getState) => {
        const me = getCurrentUserId(getState());
        return dispatch(deletePreferences(me, [{user_id: me, category: COMMUNITY_NOTIFY_CATEGORY, name: communityId, value: ''}]));
    };
}
