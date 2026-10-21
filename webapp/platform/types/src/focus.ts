// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

/**
 * Antimatter: your focus mode, Do not disturb until end_at with a reason (your custom status, with 🎯). The current
 * one (active), or the last one once it ended (ended_at set, for the summary of what you missed), or all zero.
 * Times are in milliseconds.
 */
export type FocusState = {
    active: boolean;
    start_at: number;
    end_at: number;
    reason: string;
    ended_at: number;
};

/**
 * Starts focus mode: for a duration ('15m', '1h', or 'tomorrow' for 09:00 the next day in your timezone) or until
 * end_at (milliseconds), one of the two.
 */
export type FocusRequest = {
    duration?: string;
    end_at?: number;
    reason: string;
};

export type FocusSummaryChannel = {
    channel_id: string;
    team_id: string;
    type: string;
    name: string;
    display_name: string;

    // Messages from others in the window not read yet (at most 1000), and the mentions waiting there.
    count: number;
    mention_count: number;
    last_post_at: number;
    last_sender_id: string;
};

export type FocusSummaryThread = {
    post_id: string;
    channel_id: string;
    team_id: string;
    count: number;
    mention_count: number;
    last_reply_at: number;
    last_sender_id: string;
};

/** What you missed between since and until: direct and group messages first, then mentions. */
export type FocusSummary = {
    since: number;
    until: number;
    mention_count: number;
    channels: FocusSummaryChannel[];
    threads: FocusSummaryThread[];
};
