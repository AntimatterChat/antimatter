// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/**
 * Antimatter: a community's rules, which its members agree to before they post in its channels. `version` goes up
 * each time an admin saves them asking everyone to agree again (0: the community never had rules).
 * `accepted_version` and `must_accept` are about the one who asked: the latest version they agreed to, and whether
 * they must agree before they may post (the rules are on, they haven't agreed to this version, and they aren't exempt,
 * as bots and the community's admins are).
 */
export type CommunityRules = {
    team_id: string;
    enabled: boolean;
    text: string;
    version: number;
    updated_by: string;
    update_at: number;
    accepted_version: number;
    must_accept: boolean;
};

/** Antimatter: saves a community's rules; `ask_again` asks everyone to agree again. */
export type CommunityRulesRequest = {
    enabled: boolean;
    text: string;
    ask_again: boolean;
};

/** Antimatter: how many of a community's active members agreed to the current version of its rules. */
export type CommunityRulesStats = {
    team_id: string;
    version: number;
    accepted_count: number;
    member_count: number;
};
