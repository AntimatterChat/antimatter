// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useSelector} from 'react-redux';

import type {UserProfile} from '@mattermost/types/users';

import {getStatusForUserId, getUser} from 'mattermost-redux/selectors/entities/users';

import {displayNameIn, useCommunityScope} from 'fusion/profiles/community_profiles';

import type {GlobalState} from 'types/store';

export function useUser(userId?: string): UserProfile | undefined {
    return useSelector((state: GlobalState) => (userId ? getUser(state, userId) : undefined));
}

// useDisplayName is someone's name where they show: in a community, with their nickname there and as its admins chose
// to show people; in direct and group messages and outside communities, as the user chose ("Show people as"). The
// community is the one asked for, else the community scope around (see fusion/profiles/community_profiles).
export function useDisplayName(user?: UserProfile, teamId?: string): string {
    const scope = useCommunityScope(teamId);
    return useSelector((state: GlobalState) => displayNameIn(state, user, scope));
}

export function useUserStatus(userId?: string): string {
    return useSelector((state: GlobalState) => (userId ? getStatusForUserId(state, userId) : '')) || 'offline';
}

export function initials(name: string): string {
    return name.split(/\s+/).map((part) => part[0] || '').slice(0, 2).join('').toUpperCase();
}
