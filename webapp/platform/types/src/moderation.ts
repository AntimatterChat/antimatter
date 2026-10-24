// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/**
 * Antimatter: someone kept from speaking in a team's channels (messages, reactions, files, mentions) until a time, as
 * Discord's timeouts. Direct messages aren't affected.
 */
export type MemberTimeout = {
    team_id: string;
    user_id: string;
    until: number;
    reason: string;
    created_by: string;
    create_at: number;
};

/** Antimatter: what a team's moderators see of one of its members (the mod view). */
export type MemberModeration = {
    team_id: string;
    user_id: string;
    joined_at: number;
    timeout?: MemberTimeout;
    posts_last_day: number;
    posts_last_week: number;
};

/**
 * Antimatter: someone kept out of a community (removed from it and all its channels, and refused by every way back in)
 * until the ban ends (expire_at, 0: until it's lifted). Seen by those who may ban.
 */
export type MemberBan = {
    team_id: string;
    user_id: string;
    reason: string;
    created_by: string;
    create_at: number;
    expire_at: number;
};

/** Antimatter: a ban, with how many of the person's messages it deleted at once, or the job deleting them. */
export type MemberBanResult = {
    ban: MemberBan;
    deleted_posts: number;
    purge_job_id?: string;
};

/**
 * Antimatter: one of a community's invite links, besides its permanent invite code: its id is the code in the link.
 * It ends at expire_at (0: never) and allows max_uses (0: no limit).
 */
export type TeamInviteLink = {
    id: string;
    team_id: string;
    creator_id: string;
    create_at: number;
    expire_at: number;
    max_uses: number;
    uses: number;
};
