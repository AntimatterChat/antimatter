// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {CustomStatusDuration} from '@mattermost/types/users';

import {setCustomStatus, setStatus, unsetCustomStatus} from 'mattermost-redux/actions/users';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUser} from 'mattermost-redux/selectors/entities/users';
import {isSystemAdmin} from 'mattermost-redux/utils/user_utils';

import {emitUserLoggedOutEvent} from 'actions/global_actions';
import {makeGetCustomStatus} from 'selectors/views/custom_status';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {useDisplayName, useUserStatus} from 'fusion/hooks/users';
import {useSettings} from 'fusion/shell/settings_context';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

type Props = {
    anchor: HTMLElement | null;
    onClose: () => void;
};

const STATUSES = [
    {status: 'online', label: {id: 'fusion.status.online', defaultMessage: 'Online'}, sub: null},
    {status: 'away', label: {id: 'fusion.status.away', defaultMessage: 'Away'}, sub: null},
    {status: 'dnd', label: {id: 'fusion.status.dnd', defaultMessage: 'Do not disturb'}, sub: {id: 'fusion.status.dndSub', defaultMessage: 'Mutes notifications'}},
    {status: 'offline', label: {id: 'fusion.status.invisible', defaultMessage: 'Invisible'}, sub: {id: 'fusion.status.invisibleSub', defaultMessage: 'You appear offline but can still use Antimatter'}},
];

// Do not disturb durations, as in the classic web app. The end time is in seconds; 0 means until turned off.
function dndEnd(key: string): number {
    const now = new Date();
    switch (key) {
    case '30min': return Math.floor((now.getTime() + (30 * 6e4)) / 1000);
    case '1h': return Math.floor((now.getTime() + 36e5) / 1000);
    case '2h': return Math.floor((now.getTime() + 72e5) / 1000);
    case 'tomorrow': {
        const t = new Date(now);
        t.setDate(t.getDate() + 1);
        t.setHours(9, 0, 0, 0);
        return Math.floor(t.getTime() / 1000);
    }
    default: return 0;
    }
}

const CLEAR_AFTER: Array<[CustomStatusDuration, {id: string; defaultMessage: string}]> = [
    [CustomStatusDuration.THIRTY_MINUTES, {id: 'fusion.status.clear30', defaultMessage: '30 minutes'}],
    [CustomStatusDuration.ONE_HOUR, {id: 'fusion.status.clear1h', defaultMessage: '1 hour'}],
    [CustomStatusDuration.FOUR_HOURS, {id: 'fusion.status.clear4h', defaultMessage: '4 hours'}],
    [CustomStatusDuration.TODAY, {id: 'fusion.status.clearToday', defaultMessage: 'Today'}],
    [CustomStatusDuration.THIS_WEEK, {id: 'fusion.status.clearWeek', defaultMessage: 'This week'}],
    [CustomStatusDuration.DONT_CLEAR, {id: 'fusion.status.clearNever', defaultMessage: "Don't clear"}],
];

function expiresAt(duration: CustomStatusDuration): string | undefined {
    const now = new Date();
    switch (duration) {
    case CustomStatusDuration.THIRTY_MINUTES: return new Date(now.getTime() + (30 * 6e4)).toISOString();
    case CustomStatusDuration.ONE_HOUR: return new Date(now.getTime() + 36e5).toISOString();
    case CustomStatusDuration.FOUR_HOURS: return new Date(now.getTime() + (4 * 36e5)).toISOString();
    case CustomStatusDuration.TODAY: {
        const end = new Date(now);
        end.setHours(23, 59, 59, 999);
        return end.toISOString();
    }
    case CustomStatusDuration.THIS_WEEK: {
        const end = new Date(now);
        end.setDate(end.getDate() + ((7 - end.getDay()) % 7));
        end.setHours(23, 59, 59, 999);
        return end.toISOString();
    }
    default: return undefined;
    }
}

// StatusPopover sets your status and custom status, and leads to your settings, from your avatar.
export default function StatusPopover({anchor, onClose}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const settings = useSettings();
    const toast = useToast();
    const me = useSelector(getCurrentUser);
    const team = useSelector(getCurrentTeam);
    const name = useDisplayName(me);
    const status = useUserStatus(me?.id);
    const getCustomStatus = useMemo(() => makeGetCustomStatus(), []);
    const custom = useSelector((state: GlobalState) => getCustomStatus(state, me?.id));
    const [text, setText] = useState(custom?.text || '');
    const [duration, setDuration] = useState<CustomStatusDuration>(custom?.duration || CustomStatusDuration.TODAY);

    if (!me) {
        return null;
    }
    const admin = isSystemAdmin(me.roles);
    const label = (s: string) => formatMessage(STATUSES.find((x) => x.status === s)?.label || STATUSES[3].label);

    const saveCustom = (e: React.FormEvent) => {
        e.preventDefault();
        if (text.trim() === (custom?.text || '')) {
            return;
        }
        if (text.trim()) {
            dispatch(setCustomStatus({emoji: custom?.emoji || 'speech_balloon', text: text.trim(), duration, expires_at: expiresAt(duration)}));
            toast(formatMessage({id: 'fusion.toast.customStatus', defaultMessage: 'Custom status updated'}));
        } else {
            dispatch(unsetCustomStatus());
            toast(formatMessage({id: 'fusion.toast.customStatusCleared', defaultMessage: 'Custom status cleared'}));
        }
    };

    const choose = (s: string, endTime = 0) => {
        dispatch(setStatus({user_id: me.id, status: s, dnd_end_time: endTime}));
        toast(formatMessage({id: 'fusion.toast.status', defaultMessage: 'Status set to {status}'}, {status: label(s)}));
        onClose();
    };

    return (
        <Popover
            anchor={anchor}
            placement={anchor?.closest('.am-appbar') ? 'beside' : 'below'}
            className='status-pop'
            label={formatMessage({id: 'fusion.status.label', defaultMessage: 'Set your status'})}
            onClose={onClose}
        >
            <div className={am('sp-head')}>
                <Avatar
                    userId={me.id}
                    size='lg'
                    status={true}
                />
                <div>
                    <b>{name}</b>
                    <span>{`@${me.username} · ${label(status)}`}</span>
                </div>
            </div>
            <form
                className={am('sp-custom')}
                onSubmit={saveCustom}
            >
                <Icon
                    name='smile'
                    size='sm'
                />
                <input
                    value={text}
                    placeholder={formatMessage({id: 'fusion.status.custom', defaultMessage: 'Set a custom status'})}
                    aria-label={formatMessage({id: 'fusion.status.customLabel', defaultMessage: 'Custom status'})}
                    maxLength={100}
                    onChange={(e) => setText(e.target.value)}
                    onBlur={saveCustom}
                />
                {custom?.text && (
                    <button
                        type='button'
                        aria-label={formatMessage({id: 'fusion.status.clear', defaultMessage: 'Clear custom status'})}
                        onClick={() => {
                            setText('');
                            dispatch(unsetCustomStatus());
                            toast(formatMessage({id: 'fusion.toast.customStatusCleared', defaultMessage: 'Custom status cleared'}));
                        }}
                    >
                        <Icon
                            name='x'
                            size='xs'
                        />
                    </button>
                )}
            </form>
            {text.trim() && (
                <div
                    className={am('sp-dnd')}
                    role='group'
                    aria-label={formatMessage({id: 'fusion.status.clearAfter', defaultMessage: 'Clear after'})}
                >
                    <span className={am('dur-label')}>{formatMessage({id: 'fusion.status.clearAfter', defaultMessage: 'Clear after'})}</span>
                    {CLEAR_AFTER.map(([d, l]) => (
                        <button
                            key={d || 'never'}
                            type='button'
                            className={am({on: duration === d})}
                            aria-pressed={duration === d}
                            onClick={() => {
                                setDuration(d);
                                dispatch(setCustomStatus({emoji: custom?.emoji || 'speech_balloon', text: text.trim(), duration: d, expires_at: expiresAt(d)}));
                            }}
                        >
                            {formatMessage(l)}
                        </button>
                    ))}
                </div>
            )}
            <div className={am('sp-list')}>
                {STATUSES.map((s) => {
                    const on = status === s.status;
                    const option = (
                        <button
                            className={am('sp-opt', {on})}
                            aria-pressed={on}
                            aria-haspopup={s.status === 'dnd' ? 'menu' : undefined}
                            onClick={() => choose(s.status)}
                        >
                            <span className={am('status-dot', s.status)}/>
                            <span className={am('grow')}>
                                <b>{formatMessage(s.label)}</b>
                                {s.sub && <span>{formatMessage(s.sub)}</span>}
                            </span>
                            {on && (
                                <Icon
                                    name='check'
                                    size='sm'
                                />
                            )}
                            {s.status === 'dnd' && (
                                <span className={am('chev')}>
                                    <Icon
                                        name='chev'
                                        size='xs'
                                    />
                                </span>
                            )}
                        </button>
                    );
                    if (s.status !== 'dnd') {
                        return <React.Fragment key={s.status}>{option}</React.Fragment>;
                    }
                    return (
                        <div
                            key={s.status}
                            className={am('sp-item')}
                        >
                            {option}
                            <div
                                className={am('sp-sub')}
                                role='menu'
                                aria-label={formatMessage({id: 'fusion.status.dndFor', defaultMessage: 'Do not disturb for'})}
                            >
                                {[['30min', {id: 'fusion.status.for30', defaultMessage: 'For 30 minutes'}], ['1h', {id: 'fusion.status.for1h', defaultMessage: 'For 1 hour'}], ['2h', {id: 'fusion.status.for2h', defaultMessage: 'For 2 hours'}], ['tomorrow', {id: 'fusion.status.forTomorrow', defaultMessage: 'Until tomorrow 9:00'}], ['forever', {id: 'fusion.status.forever', defaultMessage: "Don't clear"}]].map(([key, l]) => (
                                    <button
                                        key={key as string}
                                        role='menuitem'
                                        onClick={() => choose('dnd', dndEnd(key as string))}
                                    >
                                        {formatMessage(l as {id: string; defaultMessage: string})}
                                    </button>
                                ))}
                            </div>
                        </div>
                    );
                })}
            </div>
            <div className={am('sp-links')}>
                <button
                    onClick={() => {
                        settings.open('account');
                        onClose();
                    }}
                >
                    <Icon
                        name='cog'
                        size='sm'
                    />
                    {formatMessage({id: 'fusion.status.settings', defaultMessage: 'Profile & settings'})}
                </button>
                {team && (
                    <button onClick={() => getHistory().push(`/${team.name}/integrations`)}>
                        <Icon
                            name='plug'
                            size='sm'
                        />
                        {formatMessage({id: 'fusion.status.integrations', defaultMessage: 'Integrations'})}
                    </button>
                )}
                {admin && (
                    <button onClick={() => getHistory().push('/admin_console')}>
                        <Icon
                            name='shield'
                            size='sm'
                        />
                        {formatMessage({id: 'fusion.status.console', defaultMessage: 'System Console'})}
                    </button>
                )}
                <button
                    className={am('danger')}
                    onClick={() => emitUserLoggedOutEvent()}
                >
                    <Icon
                        name='export'
                        size='sm'
                    />
                    {formatMessage({id: 'fusion.status.logout', defaultMessage: 'Log out'})}
                </button>
            </div>
        </Popover>
    );
}
