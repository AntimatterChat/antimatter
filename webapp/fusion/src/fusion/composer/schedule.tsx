// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {DateTime} from 'luxon';
import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useSelector} from 'react-redux';
import {Link} from 'react-router-dom';

import {showChannelOrThreadScheduledPostIndicator} from 'mattermost-redux/selectors/entities/scheduled_posts';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentTimezone} from 'mattermost-redux/selectors/entities/timezone';

import Icon from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {MenuHeading, MenuItem, MenuSeparator} from 'fusion/components/menu';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// useMyZone is the timezone your times are in: your Mattermost timezone, else the browser's.
export function useMyZone(): string {
    return useSelector(getCurrentTimezone) || DateTime.local().zoneName || 'UTC';
}

// Quick times, as in the classic web app: tomorrow and next Monday at 9:00, in your timezone.
function quickTimes(zone: string): Array<['tomorrow' | 'monday', number]> {
    const now = DateTime.now().setZone(zone);
    const tomorrow = now.plus({days: 1}).set({hour: 9, minute: 0, second: 0, millisecond: 0});
    const monday = now.plus({days: ((8 - now.weekday) % 7) || 7}).set({hour: 9, minute: 0, second: 0, millisecond: 0});
    const times: Array<['tomorrow' | 'monday', number]> = [['tomorrow', tomorrow.toMillis()]];
    if (monday.toMillis() !== tomorrow.toMillis()) {
        times.push(['monday', monday.toMillis()]);
    }
    return times;
}

type Props = {
    anchor: HTMLElement | null;
    onSchedule: (at: number) => void;
    onClose: () => void;
};

// SchedulePopover picks when to send the message being written: the mockup's "Schedule message", backed by
// Mattermost's scheduled messages.
export function SchedulePopover({anchor, onSchedule, onClose}: Props) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const zone = useMyZone();
    const [custom, setCustom] = useState(() => DateTime.now().setZone(zone).plus({hours: 1}).startOf('hour').toFormat("yyyy-MM-dd'T'HH:mm"));
    const at = DateTime.fromISO(custom, {zone});
    const valid = at.isValid && at.toMillis() > Date.now();
    const when = (ms: number) => intl.formatDate(ms, {weekday: 'short', hour: 'numeric', minute: '2-digit', timeZone: zone});

    return (
        <Popover
            anchor={anchor}
            placement='above'
            className='plus-pop sched-pop'
            role='menu'
            label={formatMessage({id: 'fusion.schedule.label', defaultMessage: 'Schedule message'})}
            onClose={onClose}
        >
            <MenuHeading>{formatMessage({id: 'fusion.schedule.label', defaultMessage: 'Schedule message'})}</MenuHeading>
            {quickTimes(zone).map(([key, ms]) => (
                <MenuItem
                    key={key}
                    icon='clock'
                    label={key === 'tomorrow' ? formatMessage({id: 'fusion.schedule.tomorrow', defaultMessage: 'Tomorrow at 9:00'}) : formatMessage({id: 'fusion.schedule.monday', defaultMessage: 'Monday at 9:00'})}
                    sub={when(ms)}
                    onClick={() => onSchedule(ms)}
                />
            ))}
            <MenuSeparator/>
            <form
                className={am('sched-custom')}
                onSubmit={(e) => {
                    e.preventDefault();
                    if (valid) {
                        onSchedule(at.toMillis());
                    }
                }}
            >
                <label>
                    <span>{formatMessage({id: 'fusion.schedule.custom', defaultMessage: 'Custom time'})}</span>
                    <input
                        type='datetime-local'
                        value={custom}
                        onChange={(e) => setCustom(e.target.value)}
                    />
                </label>
                <button
                    className={am('btn', 'primary')}
                    disabled={!valid}
                >
                    {formatMessage({id: 'fusion.schedule.send', defaultMessage: 'Schedule'})}
                </button>
            </form>
        </Popover>
    );
}

// ScheduledNote tells that the conversation (or thread) has scheduled messages, linking to the list of them.
export function ScheduledNote({id}: {id: string}) {
    const {formatMessage} = useIntl();
    const team = useSelector(getCurrentTeam);
    const data = useSelector((state: GlobalState) => {
        const d = showChannelOrThreadScheduledPostIndicator(state, id);
        return d ? {count: d.count, at: d.scheduledPost?.scheduled_at} : null;
    }, shallowEqual);
    const intl = useIntl();
    if (!data || !team) {
        return null;
    }
    return (
        <div
            className={am('sched-note')}
            role='note'
        >
            <Icon
                name='clock'
                size='sm'
            />
            <span>
                {data.count === 1 && data.at ? formatMessage({id: 'fusion.schedule.one', defaultMessage: 'Message scheduled for {time}.'}, {time: intl.formatDate(data.at, {weekday: 'short', month: 'short', day: 'numeric', hour: 'numeric', minute: '2-digit'})}) : formatMessage({id: 'fusion.schedule.many', defaultMessage: '{count} scheduled messages in this conversation.'}, {count: data.count})}
            </span>
            <Link to={`/${team.name}/scheduled_posts?target_id=${id}`}>
                {formatMessage({id: 'fusion.schedule.seeAll', defaultMessage: 'See all'})}
            </Link>
        </div>
    );
}
