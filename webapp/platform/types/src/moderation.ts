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
