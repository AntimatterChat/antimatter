// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {getBool} from 'mattermost-redux/selectors/entities/preferences';

import {am} from 'fusion/utils/class_names';
import {Preferences} from 'utils/constants';

import type {GlobalState} from 'types/store';

const startOfDay = (d: Date) => new Date(d.getFullYear(), d.getMonth(), d.getDate()).getTime();

export function daysAgo(timestamp: number): number {
    return Math.round((startOfDay(new Date()) - startOfDay(new Date(timestamp))) / 864e5);
}

function useClockOptions(): Intl.DateTimeFormatOptions {
    const militaryTime = useSelector((state: GlobalState) => getBool(state, Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.USE_MILITARY_TIME, false));
    return {hour: '2-digit', minute: '2-digit', hour12: !militaryTime};
}

// useClock formats a timestamp as a clock time, following the user's 12/24-hour setting.
export function useClock() {
    const {formatTime} = useIntl();
    const options = useClockOptions();
    return (timestamp: number) => formatTime(timestamp, options);
}

// useWhen describes a moment the way the mockup does: "14:03", "Yesterday at 14:03", "Sep 3 at 14:03".
export function useWhen() {
    const {formatDate, formatMessage} = useIntl();
    const clock = useClock();
    return (timestamp: number) => {
        const days = daysAgo(timestamp);
        if (days === 0) {
            return clock(timestamp);
        }
        if (days === 1) {
            return formatMessage({id: 'fusion.time.yesterdayAt', defaultMessage: 'Yesterday at {time}'}, {time: clock(timestamp)});
        }
        const sameYear = new Date(timestamp).getFullYear() === new Date().getFullYear();
        const day = formatDate(timestamp, sameYear ? {month: 'short', day: 'numeric'} : {month: 'short', day: 'numeric', year: 'numeric'});
        return formatMessage({id: 'fusion.time.dayAt', defaultMessage: '{day} at {time}'}, {day, time: clock(timestamp)});
    };
}

type Props = {
    timestamp: number;
    short?: boolean;
    className?: string;
};

// MessageTime is a message's time; its title gives the full date.
export default function MessageTime({timestamp, short, className}: Props) {
    const {formatDate} = useIntl();
    const clock = useClock();
    const when = useWhen();
    const full = formatDate(timestamp, {weekday: 'long', year: 'numeric', month: 'long', day: 'numeric'}) + ' · ' + clock(timestamp);
    return (
        <time
            className={className || am('time')}
            dateTime={new Date(timestamp).toISOString()}
            title={full}
        >
            {short ? clock(timestamp) : when(timestamp)}
        </time>
    );
}

// DayLabel names a day for the separators between messages: Today, Yesterday, a weekday, or a date.
export function useDayLabel() {
    const {formatDate, formatMessage} = useIntl();
    return (timestamp: number) => {
        const days = daysAgo(timestamp);
        if (days === 0) {
            return formatMessage({id: 'fusion.time.today', defaultMessage: 'Today'});
        }
        if (days === 1) {
            return formatMessage({id: 'fusion.time.yesterday', defaultMessage: 'Yesterday'});
        }
        if (days < 7) {
            return formatDate(timestamp, {weekday: 'long'});
        }
        const sameYear = new Date(timestamp).getFullYear() === new Date().getFullYear();
        return formatDate(timestamp, sameYear ? {weekday: 'long', month: 'long', day: 'numeric'} : {weekday: 'long', month: 'long', day: 'numeric', year: 'numeric'});
    };
}
