// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/**
 * Antimatter: what a role of the channel's team (target_type 'role', a group id) or one person (target_type 'user')
 * may do in a channel on top of, or less than, what they have elsewhere.
 */
export type ChannelOverride = {
    channel_id: string;
    target_type: 'role' | 'user';
    target_id: string;
    allow: string[];
    deny: string[];
    update_at: number;
};

/**
 * Antimatter: an override set on a server category (category: its name, trimmed and in lower case). The channels filed
 * in the category inherit it; a channel's own override wins over it, permission by permission.
 */
export type CategoryOverride = {
    team_id: string;
    category: string;
    target_type: 'role' | 'user';
    target_id: string;
    allow: string[];
    deny: string[];
    update_at: number;
};
