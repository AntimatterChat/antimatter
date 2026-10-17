// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import type {IntlShape} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';
import type {OnTopPost} from '@mattermost/types/posts';

import {fetchIsRestrictedDM} from 'mattermost-redux/actions/channels';
import {Permissions} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getOnTopEntry, isOnTopActive} from 'mattermost-redux/selectors/entities/on_top_posts';
import {haveIChannelPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// Messages put on top of a channel (Antimatter): shown in a strip above the channel's messages, each for a while or
// until removed, and tagged "On top" in the conversation.

export type OnTopDuration = '1h' | '4h' | '1d' | '1w' | 'forever';

const HOUR = 3600000;

// The times a message can be put on top for, as the mockup's: 0 is until removed.
export const ON_TOP_DURATIONS: Array<[OnTopDuration, number]> = [
    ['1h', HOUR],
    ['4h', 4 * HOUR],
    ['1d', 24 * HOUR],
    ['1w', 7 * 24 * HOUR],
    ['forever', 0],
];

// How often the time left is shown again.
const TICK_MS = 30000;
const MAX_TIMER_MS = 2147483647;

// activeOnTop leaves out the messages whose time on top ended.
export function activeOnTop(list: OnTopPost[], now: number): OnTopPost[] {
    return list.filter((o) => isOnTopActive(o, now));
}

// nextExpiry is when the first of the messages still on top expires, if any does.
export function nextExpiry(list: OnTopPost[], now: number): number | undefined {
    let next: number | undefined;
    for (const o of list) {
        if (o.expires_at > now && (next === undefined || o.expires_at < next)) {
            next = o.expires_at;
        }
    }
    return next;
}

// timeLeft tells how long a message stays on top, as the mockup's: "2d 3h left", "3h 20m left", "5m left", or
// "until removed".
export function timeLeft(intl: IntlShape, expiresAt: number, now: number): string {
    if (!expiresAt) {
        return intl.formatMessage({id: 'fusion.ontop.left.forever', defaultMessage: 'until removed'});
    }
    const minutes = Math.round(Math.max(0, expiresAt - now) / 60000);
    if (minutes >= 1440) {
        return intl.formatMessage({id: 'fusion.ontop.left.days', defaultMessage: '{days}d {hours}h left'}, {days: Math.floor(minutes / 1440), hours: Math.floor((minutes % 1440) / 60)});
    }
    if (minutes >= 60) {
        return intl.formatMessage({id: 'fusion.ontop.left.hours', defaultMessage: '{hours}h {minutes}m left'}, {hours: Math.floor(minutes / 60), minutes: minutes % 60});
    }
    return intl.formatMessage({id: 'fusion.ontop.left.minutes', defaultMessage: '{minutes}m left'}, {minutes: Math.max(1, minutes)});
}

export function durationLabel(intl: IntlShape, duration: OnTopDuration): string {
    switch (duration) {
    case '1h':
        return intl.formatMessage({id: 'fusion.ontop.duration.1h', defaultMessage: 'For 1 hour'});
    case '4h':
        return intl.formatMessage({id: 'fusion.ontop.duration.4h', defaultMessage: 'For 4 hours'});
    case '1d':
        return intl.formatMessage({id: 'fusion.ontop.duration.1d', defaultMessage: 'For 1 day'});
    case '1w':
        return intl.formatMessage({id: 'fusion.ontop.duration.1w', defaultMessage: 'For 1 week'});
    default:
        return intl.formatMessage({id: 'fusion.ontop.duration.forever', defaultMessage: 'Until removed'});
    }
}

// isRestrictedDMUnknown tells whether a direct or group message might be restricted (direct messages only between people
// sharing a team) without knowing yet whether it is.
function isRestrictedDMUnknown(state: GlobalState, channel: Channel): boolean {
    return (channel.type === 'D' || channel.type === 'G') && getConfig(state).RestrictDirectMessage === 'team' &&
        state.entities.channels.restrictedDMs?.[channel.id] === undefined;
}

// canChangeOnTop tells whether one may put messages on top of a channel and take them off, as the server does
// (SessionCanChangeOnTop): not in an archived channel or a restricted direct message (nor while it's not known whether
// it's restricted); in a team channel only its admins, the community's admins and moderators (who manage messages),
// where they may post (its moderation, overrides, a timeout and membership apply). Members don't.
export function canChangeOnTop(state: GlobalState, channel: Channel | undefined): boolean {
    if (!channel || channel.delete_at !== 0) {
        return false;
    }
    if (channel.type === 'D' || channel.type === 'G') {
        if (isRestrictedDMUnknown(state, channel)) {
            return false;
        }
        return !state.entities.channels.restrictedDMs?.[channel.id];
    }
    if (!haveIChannelPermission(state, channel.team_id, channel.id, Permissions.CREATE_POST)) {
        return false;
    }
    return haveIChannelPermission(state, channel.team_id, channel.id, Permissions.MANAGE_CHANNEL_ROLES) ||
        haveIChannelPermission(state, channel.team_id, channel.id, Permissions.DELETE_OTHERS_POSTS) ||
        haveITeamPermission(state, channel.team_id, Permissions.MANAGE_TEAM);
}

// useCanChangeOnTop is canChangeOnTop, finding out first whether a direct message is restricted when it's not known.
export function useCanChangeOnTop(channel: Channel | undefined): boolean {
    const dispatch = useDispatch();
    const unknown = useSelector((state: GlobalState) => Boolean(channel) && isRestrictedDMUnknown(state, channel!));
    const channelId = channel?.id;
    useEffect(() => {
        if (unknown && channelId) {
            dispatch(fetchIsRestrictedDM(channelId));
        }
    }, [unknown, channelId, dispatch]);
    return useSelector((state: GlobalState) => canChangeOnTop(state, channel));
}

// useNow is the time, again every half minute, and when a deadline comes (the next expiry), so that what's shown
// follows the clock.
export function useNow(deadline?: number): number {
    const [now, setNow] = useState(Date.now);
    useEffect(() => {
        const interval = setInterval(() => setNow(Date.now()), TICK_MS);
        return () => clearInterval(interval);
    }, []);
    useEffect(() => {
        // Timers can't wait longer than about 24 days; the half-minute ticks get there.
        if (!deadline || deadline - Date.now() > MAX_TIMER_MS) {
            return undefined;
        }

        // A little after the deadline, so that the expired message is surely left out.
        const timer = setTimeout(() => setNow(Date.now()), Math.max(0, deadline - Date.now()) + 250);
        return () => clearTimeout(timer);
    }, [deadline]);
    return now;
}

// OnTopTag is the "On top · 3h left" tag of a message on top of its channel, among its meta tags.
export function OnTopTag({channelId, postId}: {channelId: string; postId: string}) {
    const entry = useSelector((state: GlobalState) => getOnTopEntry(state, channelId, postId));
    if (!entry) {
        return null;
    }
    return <OnTopTagLabel entry={entry}/>;
}

function OnTopTagLabel({entry}: {entry: OnTopPost}) {
    const intl = useIntl();
    const now = useNow(entry.expires_at || undefined);
    if (!isOnTopActive(entry, now)) {
        return null;
    }
    return (
        <span
            className={am('meta-tag', 'ontop-tag')}
            title={intl.formatMessage({id: 'fusion.ontop.tagTitle', defaultMessage: 'On top of the channel'})}
        >
            <Icon name='pin-top'/>
            {intl.formatMessage({id: 'fusion.ontop.tag', defaultMessage: 'On top · {left}'}, {left: timeLeft(intl, entry.expires_at, now)})}
        </span>
    );
}
