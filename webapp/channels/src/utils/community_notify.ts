// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {UserNotifyProps, UserProfile} from '@mattermost/types/users';

import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';

import type {GlobalState} from 'types/store';

// Notification settings per community (Antimatter; set in the Fusion UI, from a community's menu): a community_notify
// preference, named after the community's id, holds the notify props set over the person's own for that community's
// channels. The server applies them to push and email notifications; the desktop notifications of this UI follow them
// too. A channel's own notification preferences still win; direct and group messages follow the person's own settings.
const KEYS = ['desktop', 'desktop_threads', 'desktop_sound', 'desktop_notification_sound', 'push', 'push_threads', 'email', 'channel'];

export function getCommunityNotifyProps(state: GlobalState, communityId: string): Partial<UserNotifyProps> {
    const raw = communityId ? getPreference(state, 'community_notify', communityId, '') : '';
    const props: Record<string, string> = {};
    if (raw) {
        try {
            const value = JSON.parse(raw);
            for (const key of KEYS) {
                if (value && typeof value[key] === 'string' && value[key]) {
                    props[key] = value[key];
                }
            }
        } catch {
            // Settings that can't be read are none.
        }
    }
    return props as Partial<UserNotifyProps>;
}

export function withCommunityNotifyProps(user: UserProfile, props: Partial<UserNotifyProps>): UserProfile {
    if (!user || !Object.keys(props).length) {
        return user;
    }
    return {...user, notify_props: {...user.notify_props, ...props}};
}
