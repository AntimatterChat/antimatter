// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {DateTime} from 'luxon';
import React, {useEffect, useMemo, useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useSelector} from 'react-redux';

import type {UserProfile} from '@mattermost/types/users';

import {getChannel, getDirectTeammate} from 'mattermost-redux/selectors/entities/channels';
import {getTeammateNameDisplaySetting} from 'mattermost-redux/selectors/entities/preferences';
import {isScheduledPostsEnabled} from 'mattermost-redux/selectors/entities/scheduled_posts';
import {getCurrentUserId, makeGetProfilesInChannel} from 'mattermost-redux/selectors/entities/users';
import {getUserCurrentTimezone} from 'mattermost-redux/utils/timezone_utils';
import {displayUsername} from 'mattermost-redux/utils/user_utils';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';
import Constants from 'utils/constants';

import type {GlobalState} from 'types/store';

// The hours when someone may be asleep, as the classic web app's "remote user hour" (22:00 to 06:00).
const NIGHT_FROM = Constants.REMOTE_USERS_HOUR_LIMIT_END_OF_THE_DAY;
const NIGHT_TO = Constants.REMOTE_USERS_HOUR_LIMIT_BEGINNING_OF_THE_DAY;

// The conversations where the note was dismissed, until the page reloads (as in the mockup).
const dismissed = new Set<string>();

type Sleeper = {user: UserProfile; time: DateTime};

function sleepers(users: UserProfile[], now: DateTime): Sleeper[] {
    const out: Sleeper[] = [];
    for (const user of users) {
        const zone = getUserCurrentTimezone(user.timezone);
        if (!zone || user.is_bot) {
            continue;
        }
        const time = now.setZone(zone);
        if (time.isValid && (time.hour >= NIGHT_FROM || time.hour < NIGHT_TO)) {
            out.push({user, time});
        }
    }
    return out;
}

type Props = {
    channelId: string;

    // Schedules the message being written; missing when scheduled messages are off.
    onSendAt?: (at: number, sleeper: string) => void;
};

// SleepNote warns, in a direct or group message, that the others may be asleep: the mockup's .sleep-note, with
// "Send at 07:00 their time" for one sleeper.
export default function SleepNote({channelId, onSendAt}: Props) {
    const {formatMessage, formatTime} = useIntl();
    const getProfiles = useMemo(() => makeGetProfilesInChannel(), []);
    const channel = useSelector((state: GlobalState) => getChannel(state, channelId));
    const me = useSelector(getCurrentUserId);
    const nameDisplay = useSelector(getTeammateNameDisplaySetting);
    const scheduling = useSelector(isScheduledPostsEnabled);

    // A direct message's teammate is always known; a group's members once loaded.
    const others = useSelector((state: GlobalState) => {
        const teammate = getDirectTeammate(state, channelId);
        if (teammate) {
            return teammate.id === me ? [] : [teammate];
        }
        return getProfiles(state, channelId, {active: true}).filter((u) => u.id !== me);
    }, shallowEqual);
    const [now, setNow] = useState(DateTime.now);
    const [, setDismissed] = useState(0);

    useEffect(() => {
        const timer = setInterval(() => setNow(DateTime.now()), 60000);
        return () => clearInterval(timer);
    }, []);

    if (!channel || (channel.type !== Constants.DM_CHANNEL && channel.type !== Constants.GM_CHANNEL) || dismissed.has(channelId)) {
        return null;
    }
    const asleep = sleepers(others, now);
    if (!asleep.length) {
        return null;
    }
    const first = (u: UserProfile) => u.first_name || displayUsername(u, nameDisplay);

    let text;
    let later = null;
    if (asleep.length === 1 && others.length === 1) {
        const [{user, time}] = asleep;
        text = formatMessage(
            {id: 'fusion.sleep.one', defaultMessage: "It's <b>{time}</b> for {name} — they may be asleep."},
            {time: formatTime(time.toMillis(), {timeZone: time.zoneName || undefined, hour: '2-digit', minute: '2-digit'}), name: first(user), b: (chunks: React.ReactNode) => <b>{chunks}</b>},
        );
        if (scheduling && onSendAt) {
            let wake = time.set({hour: 7, minute: 0, second: 0, millisecond: 0});
            if (wake <= time) {
                wake = wake.plus({days: 1});
            }
            later = (
                <button
                    type='button'
                    className={am('sleep-later')}
                    onClick={() => onSendAt(wake.toMillis(), first(user))}
                >
                    {formatMessage({id: 'fusion.sleep.later', defaultMessage: 'Send at 07:00 their time'})}
                </button>
            );
        }
    } else {
        text = formatMessage({id: 'fusion.sleep.many', defaultMessage: "It's night for {names} — they may be asleep."}, {names: asleep.map((s) => first(s.user)).join(', ')});
    }

    return (
        <div
            className={am('sleep-note')}
            role='note'
        >
            <Icon
                name='moon'
                size='sm'
            />
            <span>{text}</span>
            {later}
            <button
                type='button'
                className={am('sleep-x')}
                aria-label={formatMessage({id: 'fusion.sleep.dismiss', defaultMessage: 'Dismiss'})}
                title={formatMessage({id: 'fusion.sleep.dismiss', defaultMessage: 'Dismiss'})}
                onClick={() => {
                    dismissed.add(channelId);
                    setDismissed((n) => n + 1);
                }}
            >
                <Icon
                    name='x'
                    size='xs'
                />
            </button>
        </div>
    );
}
