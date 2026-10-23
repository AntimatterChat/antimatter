// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {createContext, useCallback, useContext} from 'react';
import {useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';
import type {CommunityProfile} from '@mattermost/types/community_profiles';
import type {UserProfile} from '@mattermost/types/users';

import {Client4} from 'mattermost-redux/client';
import {General, Preferences} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getTeammateNameDisplaySetting} from 'mattermost-redux/selectors/entities/preferences';
import {getTeam} from 'mattermost-redux/selectors/entities/teams';
import {displayUsername} from 'mattermost-redux/utils/user_utils';

import {imageURLForUser} from 'utils/utils';

import type {GlobalState} from 'types/store';

// Community profiles (Antimatter): in a community, people show with their nickname and picture there (when they set
// them) instead of their global ones, and as the community's admins chose to show people (username, nickname or full
// name). Direct and group messages, and everything outside communities, keep the global profile and each person's own
// choice of how to show people. Whoever hides their full name and nickname from a community (its privacy settings)
// comes without them from the server, and so shows as their username (or their nickname there, chosen for it).
//
// What applies where is the "community scope": a community's id, or '' for the global profile. Components showing
// someone take it from the CommunityScope around them (a message gives its channel's), else from the conversation
// open (its community, '' for a direct or group message).

export const CommunityScopeContext = createContext<string | undefined>(undefined);

/** CommunityScope sets the community whose profiles apply inside it ('' for the global profile). */
export const CommunityScope = CommunityScopeContext.Provider;

/** scopeOfChannel is the community scope of a channel's messages: its community, '' for direct and group messages. */
export function scopeOfChannel(channel?: Channel): string | undefined {
    if (!channel) {
        return undefined;
    }
    if (channel.type === General.DM_CHANNEL || channel.type === General.GM_CHANNEL) {
        return '';
    }
    return channel.team_id || '';
}

/** getCurrentScope is the community scope of the conversation open, else of the current community. */
export function getCurrentScope(state: GlobalState): string {
    const channels = state.entities.channels;
    const scope = scopeOfChannel(channels?.channels?.[channels.currentChannelId]);
    return scope === undefined ? (state.entities.teams?.currentTeamId || '') : scope;
}

/** useCommunityScope gives the community scope: the one asked for, else the one around, else the current one. */
export function useCommunityScope(teamId?: string): string {
    const around = useContext(CommunityScopeContext);
    const current = useSelector(getCurrentScope);
    return teamId ?? around ?? current;
}

/** getCommunityProfile is someone's profile in a community, when they set one and the community's were loaded. */
export function getCommunityProfile(state: GlobalState, teamId: string, userId?: string): CommunityProfile | undefined {
    if (!teamId || !userId) {
        return undefined;
    }
    return state.entities.teams.communityProfiles?.[teamId]?.[userId];
}

/**
 * getNameDisplayIn is how people show in a community scope: as the community's admins chose, else as the server's
 * default; outside communities, as the person chose (their "Show people as").
 */
export function getNameDisplayIn(state: GlobalState, teamId: string): string {
    if (!teamId) {
        return getTeammateNameDisplaySetting(state);
    }
    const chosen = getTeam(state, teamId)?.name_display;
    if (chosen) {
        return chosen;
    }
    return getConfig(state).TeammateNameDisplay || Preferences.DISPLAY_PREFER_USERNAME;
}

/** communityView is someone as a community shows them: with their nickname there instead of their global one. */
export function communityView(user: UserProfile, profile?: CommunityProfile): UserProfile {
    if (!profile?.nickname) {
        return user;
    }
    return {...user, nickname: profile.nickname};
}

/** displayNameIn is someone's name in a community scope. */
export function displayNameIn(state: GlobalState, user: UserProfile | undefined, teamId: string): string {
    if (!user) {
        return displayUsername(user, getNameDisplayIn(state, teamId));
    }
    return displayUsername(communityView(user, getCommunityProfile(state, teamId, user.id)), getNameDisplayIn(state, teamId));
}

/** globalNameOf is someone's name outside communities, as the current user shows people there. */
export function globalNameOf(state: GlobalState, user: UserProfile | undefined): string {
    return displayUsername(user, getTeammateNameDisplaySetting(state));
}

/** avatarURLIn is someone's picture in a community scope: theirs for the community when they set one. */
export function avatarURLIn(user: UserProfile, profile: CommunityProfile | undefined, teamId: string): string {
    if (teamId && profile?.avatar_update_at) {
        return Client4.getCommunityAvatarUrl(teamId, user.id, profile.avatar_update_at);
    }
    return imageURLForUser(user.id, user.last_picture_update);
}

/** useCommunityProfile gives someone's profile in the community scope (or the one asked for). */
export function useCommunityProfile(userId?: string, teamId?: string): [CommunityProfile | undefined, string] {
    const scope = useCommunityScope(teamId);
    const profile = useSelector((state: GlobalState) => getCommunityProfile(state, scope, userId));
    return [profile, scope];
}

const NO_PROFILES: Record<string, CommunityProfile> = {};

/** useDisplayNames gives a function naming people in the community scope (or the one asked for), for lists. */
export function useDisplayNames(teamId?: string): (user?: UserProfile) => string {
    const scope = useCommunityScope(teamId);
    const setting = useSelector((state: GlobalState) => getNameDisplayIn(state, scope));
    const profiles = useSelector((state: GlobalState) => (scope && state.entities.teams.communityProfiles?.[scope]) || NO_PROFILES);
    return useCallback((user?: UserProfile) => displayUsername(user ? communityView(user, profiles[user.id]) : user, setting), [setting, profiles]);
}
