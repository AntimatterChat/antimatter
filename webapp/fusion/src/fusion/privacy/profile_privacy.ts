// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {updateMe} from 'mattermost-redux/actions/users';
import {getCurrentUser} from 'mattermost-redux/selectors/entities/users';

import type {ActionFuncAsync, GlobalState} from 'types/store';

// Profile privacy: which of your profile details the people of each community see. It's your profile_privacy prop: a
// JSON object of the details hidden for all communities ("default") and, where you chose otherwise, in a community (by
// its id; an empty list there shows everything). The server enforces it: those it's hidden from never get the details,
// nor the prop. Someone who shares several communities with you sees a detail only when none of them hides it; someone
// who shares none sees what your defaults show. Your username and picture always show.
export const PROFILE_PRIVACY_PROP = 'profile_privacy';
export const PROFILE_DEFAULT = 'default';

export const PROFILE_FIELDS = ['full_name', 'nickname', 'position', 'email', 'local_time', 'attributes'] as const;
export type ProfileField = typeof PROFILE_FIELDS[number];
export type ProfilePrivacy = Record<string, ProfileField[]>;

const NONE: ProfilePrivacy = {};
const parsed = new Map<string, ProfilePrivacy>();

export function parseProfilePrivacy(raw?: string): ProfilePrivacy {
    if (!raw) {
        return NONE;
    }
    let privacy = parsed.get(raw);
    if (!privacy) {
        privacy = {};
        try {
            const value = JSON.parse(raw);
            if (value && typeof value === 'object' && !Array.isArray(value)) {
                for (const [key, fields] of Object.entries(value)) {
                    if (Array.isArray(fields)) {
                        privacy[key] = fields.filter((f): f is ProfileField => (PROFILE_FIELDS as readonly string[]).includes(f));
                    }
                }
            }
        } catch {
            // The server never lets a choice it can't read be saved.
        }
        parsed.set(raw, privacy);
    }
    return privacy;
}

export function getMyProfilePrivacy(state: GlobalState): ProfilePrivacy {
    return parseProfilePrivacy(getCurrentUser(state)?.props?.[PROFILE_PRIVACY_PROP]);
}

// hiddenIn gives the details hidden in a community: its own choice, else the defaults.
export function hiddenIn(privacy: ProfilePrivacy, communityId: string): ProfileField[] {
    return privacy[communityId] ?? privacy[PROFILE_DEFAULT] ?? [];
}

// setHiddenFields sets the details hidden for all communities (PROFILE_DEFAULT) or in one; null lets a community follow
// the defaults again.
export function setHiddenFields(communityId: string, fields: ProfileField[] | null): ActionFuncAsync {
    return async (dispatch, getState) => {
        const me = getCurrentUser(getState());
        const next: ProfilePrivacy = {...getMyProfilePrivacy(getState())};
        if (fields === null || (communityId === PROFILE_DEFAULT && !fields.length)) {
            delete next[communityId];
        } else {
            next[communityId] = PROFILE_FIELDS.filter((f) => fields.includes(f));
        }
        const props = {...me.props};
        if (Object.keys(next).length) {
            props[PROFILE_PRIVACY_PROP] = JSON.stringify(next);
        } else {
            delete props[PROFILE_PRIVACY_PROP];
        }
        return dispatch(updateMe({props}));
    };
}
