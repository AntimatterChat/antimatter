// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId, getCurrentUser} from 'mattermost-redux/selectors/entities/users';
import {getUserCurrentTimezone} from 'mattermost-redux/utils/timezone_utils';
import {isSystemAdmin} from 'mattermost-redux/utils/user_utils';

import {openDirectChannelToUserId} from 'actions/channel_actions';
import {makeGetCustomStatus} from 'selectors/views/custom_status';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {useDisplayName, useUser, useUserStatus} from 'fusion/hooks/users';
import {useSettings} from 'fusion/shell/settings_context';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

import RoleChips from './role_chips';

const STATUS_LABELS: Record<string, {id: string; defaultMessage: string}> = {
    online: {id: 'fusion.status.online', defaultMessage: 'Online'},
    away: {id: 'fusion.status.away', defaultMessage: 'Away'},
    dnd: {id: 'fusion.status.dnd', defaultMessage: 'Do not disturb'},
    offline: {id: 'fusion.status.offline', defaultMessage: 'Offline'},
};

// useLocalTime gives someone's current time and how far it is from yours.
function useLocalTime(timezone?: string): {time: string; rel: string} | null {
    const {formatTime, formatMessage} = useIntl();
    const me = useSelector(getCurrentUser);
    if (!timezone) {
        return null;
    }
    const now = new Date();
    const offset = (tz: string) => {
        const there = new Date(now.toLocaleString('en-US', {timeZone: tz}));
        return Math.round((there.getTime() - now.getTime()) / 18e5) / 2;
    };
    let diff = 0;
    try {
        const mine = getUserCurrentTimezone(me?.timezone);
        diff = mine ? offset(timezone) - offset(mine) : 0;
    } catch {
        return null;
    }
    const rel = diff === 0 ? formatMessage({id: 'fusion.profile.sameTime', defaultMessage: 'same time as you'}) : formatMessage({id: 'fusion.profile.timeDiff', defaultMessage: '{hours}h {ahead, select, true {ahead of} other {behind}} you'}, {hours: Math.abs(diff), ahead: String(diff > 0)});
    return {time: formatTime(now, {timeZone: timezone, hour: '2-digit', minute: '2-digit'}), rel};
}

type Props = {
    userId: string;
    anchor: HTMLElement | null;
    onClose: () => void;
};

// UserPopover is someone's profile card.
export default function UserPopover({userId, anchor, onClose}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const settings = useSettings();
    const user = useUser(userId);
    const name = useDisplayName(user);
    const status = useUserStatus(userId);
    const me = useSelector(getCurrentUserId);
    const team = useSelector(getCurrentTeam);
    const admin = useSelector((state: GlobalState) => isSystemAdmin(getCurrentUser(state)?.roles || ''));
    const getCustomStatus = useMemo(() => makeGetCustomStatus(), []);
    const customStatus = useSelector((state: GlobalState) => getCustomStatus(state, userId));
    const localTime = useLocalTime(user ? getUserCurrentTimezone(user.timezone) : undefined);

    if (!user) {
        return null;
    }
    const isMe = userId === me;
    const bot = user.is_bot;

    const message = async () => {
        const result = await dispatch(openDirectChannelToUserId(userId));
        if ('data' in result && result.data && team) {
            getHistory().push(channelPath(team.name, result.data));
        }
        onClose();
    };

    return (
        <Popover
            anchor={anchor}
            placement='beside'
            className=''
            label={name}
            onClose={onClose}
        >
            <div className={am('cover')}/>
            <div className={am('pbody')}>
                <Avatar
                    userId={userId}
                    size='xl'
                    status={true}
                />
                <h4 style={{display: 'flex', alignItems: 'center', gap: 6}}>
                    {name}
                    {bot && <span className={am('bot-tag')}>{'BOT'}</span>}
                </h4>
                <div className={am('handle')}>{`@${user.username} · ${formatMessage(STATUS_LABELS[status] || STATUS_LABELS.offline)}`}</div>
                {!bot && (user.position || localTime) && (
                    <div className={am('facts')}>
                        {user.position && (
                            <span><Icon name='users'/>{user.position}</span>
                        )}
                        {localTime && (
                            <span>
                                <Icon name='clock'/>
                                {formatMessage({id: 'fusion.profile.localTime', defaultMessage: '{time} local time'}, {time: localTime.time})}
                                {isMe ? '' : ` · ${localTime.rel}`}
                            </span>
                        )}
                    </div>
                )}
                {bot && (
                    <div className={am('facts')}>
                        <span><Icon name='plug'/>{user.bot_description || formatMessage({id: 'fusion.profile.bot', defaultMessage: 'Integration · posts on behalf of an app'})}</span>
                    </div>
                )}
                {customStatus?.text && (
                    <p
                        className={am('note')}
                        style={{marginTop: 10}}
                    >
                        {customStatus.text}
                    </p>
                )}
                <RoleChips userId={userId}/>
                <div style={{marginTop: 12}}>
                    {isMe && (
                        <div style={{display: 'grid', gap: 8}}>
                            <button
                                className={am('btn')}
                                onClick={() => {
                                    settings.open('account');
                                    onClose();
                                }}
                            >
                                <Icon
                                    name='cog'
                                    size='sm'
                                />
                                {formatMessage({id: 'fusion.profile.settings', defaultMessage: 'User settings'})}
                            </button>
                            {admin && (
                                <button
                                    className={am('btn')}
                                    onClick={() => getHistory().push('/admin_console')}
                                >
                                    <Icon
                                        name='shield'
                                        size='sm'
                                    />
                                    {formatMessage({id: 'fusion.profile.console', defaultMessage: 'System Console'})}
                                </button>
                            )}
                        </div>
                    )}
                    {!isMe && (
                        <button
                            className={am('btn', 'primary')}
                            style={{width: '100%', justifyContent: 'center'}}
                            onClick={message}
                        >
                            <Icon
                                name='chat'
                                size='sm'
                            />
                            {formatMessage({id: 'fusion.profile.message', defaultMessage: 'Message'})}
                        </button>
                    )}
                </div>
            </div>
        </Popover>
    );
}
