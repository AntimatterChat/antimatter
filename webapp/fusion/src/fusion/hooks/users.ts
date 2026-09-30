// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useSelector} from 'react-redux';

import type {UserProfile} from '@mattermost/types/users';

import {getTeammateNameDisplaySetting} from 'mattermost-redux/selectors/entities/preferences';
import {getStatusForUserId, getUser} from 'mattermost-redux/selectors/entities/users';
import {displayUsername} from 'mattermost-redux/utils/user_utils';

import type {GlobalState} from 'types/store';

export function useUser(userId?: string): UserProfile | undefined {
    return useSelector((state: GlobalState) => (userId ? getUser(state, userId) : undefined));
}

// useDisplayName follows the user's "Teammate name display" setting.
export function useDisplayName(user?: UserProfile): string {
    const setting = useSelector(getTeammateNameDisplaySetting);
    return displayUsername(user, setting);
}

export function useUserStatus(userId?: string): string {
    return useSelector((state: GlobalState) => (userId ? getStatusForUserId(state, userId) : '')) || 'offline';
}

export function initials(name: string): string {
    return name.split(/\s+/).map((part) => part[0] || '').slice(0, 2).join('').toUpperCase();
}
