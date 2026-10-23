// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/**
 * Antimatter: someone's profile in one community. Their nickname and picture there show instead of their global ones
 * in the community's channels, member list, mentions and cards (direct and group messages keep the global profile).
 * `avatar_update_at` is when they set their picture there, 0 (or missing) for none. Refusing to be shown by name in
 * a community is one of its privacy settings (its profile details), not part of the profile.
 */
export type CommunityProfile = {
    team_id: string;
    user_id: string;
    nickname?: string;
    avatar_update_at?: number;
    update_at: number;
};

/** Antimatter: what someone changes of their own community profile; an empty nickname removes it. */
export type CommunityProfilePatch = {
    nickname?: string;
};

/** Antimatter: takes a nickname and/or a picture off a community profile (one's own, or a member's for moderators). */
export type CommunityProfileReset = {
    nickname?: boolean;
    avatar?: boolean;
};

/** The way a community shows people (its admins' choice), as TeammateNameDisplay. */
export type CommunityNameDisplay = 'username' | 'nickname_full_name' | 'full_name';
