// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {getMemberTimeout} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId, getCurrentUser} from 'mattermost-redux/selectors/entities/users';
import {getUserCurrentTimezone} from 'mattermost-redux/utils/timezone_utils';

import {openDirectChannelToUserId} from 'actions/channel_actions';
import {makeGetCustomStatus} from 'selectors/views/custom_status';

import {AgentActivityLine, IntegrationTag, useAgentBusy} from 'fusion/agents/agent_status';
import CallUserButton, {useCanCall} from 'fusion/calls/call_user_button';
import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {useExclusiveCard} from 'fusion/components/layer/exclusive';
import {useDisplayName, useUser, useUserStatus} from 'fusion/hooks/users';
import {canModerateTeam, openModView} from 'fusion/moderation/moderation';
import {useSettings} from 'fusion/shell/settings_context';
import {useToast} from 'fusion/shell/toast_context';
import {useRoleColor} from 'fusion/teams/roles';
import {TeamTag} from 'fusion/teams/team_card';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import Pluggable from 'plugins/pluggable';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

import {canVisitSystemConsole} from './admin_links';
import ProfileFacts from './profile_facts';
import RoleChips from './role_chips';
import StatusPopover from './status_popover';
import {UserMenu} from './user_menu';
import UserNote from './user_note';

export const STATUS_LABELS: Record<string, {id: string; defaultMessage: string}> = {
    online: {id: 'fusion.status.online', defaultMessage: 'Online'},
    away: {id: 'fusion.status.away', defaultMessage: 'Away'},
    dnd: {id: 'fusion.status.dnd', defaultMessage: 'Do not disturb'},
    offline: {id: 'fusion.status.offline', defaultMessage: 'Offline'},
    ooo: {id: 'fusion.status.ooo', defaultMessage: 'Out of office'},
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

    // Elements that toggle the card besides its anchor (see Popover).
    toggles?: Array<HTMLElement | null>;
};

// UserPopover is someone's profile card.
export default function UserPopover({userId, anchor, onClose, toggles}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const settings = useSettings();
    const toast = useToast();
    const user = useUser(userId);
    const name = useDisplayName(user);
    const status = useUserStatus(userId);
    const me = useSelector(getCurrentUserId);
    const team = useSelector(getCurrentTeam);

    // Every role that may read a part of the System Console (system admin, system manager, user manager...).
    const systemConsole = useSelector(canVisitSystemConsole);
    const getCustomStatus = useMemo(() => makeGetCustomStatus(), []);
    const customStatus = useSelector((state: GlobalState) => getCustomStatus(state, userId));
    const localTime = useLocalTime(user ? getUserCurrentTimezone(user.timezone) : undefined);
    const canCall = useCanCall(userId === me, Boolean(user?.is_bot));
    const roleColor = useRoleColor(userId);
    useExclusiveCard(onClose);
    const statusButton = useRef<HTMLButtonElement>(null);
    const [settingStatus, setSettingStatus] = useState(false);
    const moreButton = useRef<HTMLButtonElement>(null);
    const [more, setMore] = useState(false);
    const agentBusy = useAgentBusy(user);
    const canModerate = useSelector((state: GlobalState) => Boolean(team) && canModerateTeam(state, team!.id));
    const timeout = useSelector((state: GlobalState) => (team ? getMemberTimeout(state, team.id, userId) : undefined));
    const {formatTime, formatDate} = useIntl();

    if (!user) {
        return null;
    }
    const isMe = userId === me;
    const bot = user.is_bot;
    const agent = Boolean(user.is_agent);

    const message = async () => {
        const result = await dispatch(openDirectChannelToUserId(userId));
        if ('data' in result && result.data && team) {
            getHistory().push(channelPath(team.name, result.data));
        } else if ('error' in result && result.error) {
            // Someone who blocked you, or whom you block.
            toast((result.error as {message?: string}).message || formatMessage({id: 'fusion.userMenu.messageFailed', defaultMessage: 'You can\'t message {name}.'}, {name}));
        }
        onClose();
    };

    return (
        <Popover
            anchor={anchor}
            toggles={toggles}
            placement='beside'
            className=''
            label={name}
            onClose={onClose}
        >
            <div className={am('cover')}>
                {/* On the banner, top right, as Discord's: the mod view for moderators, and the person's menu. */}
                <div className={am('cover-actions')}>
                    {canModerate && !isMe && (
                        <button
                            type='button'
                            className={am('cover-btn')}
                            title={formatMessage({id: 'fusion.profile.modView', defaultMessage: 'Mod view'})}
                            aria-label={formatMessage({id: 'fusion.profile.modView', defaultMessage: 'Mod view'})}
                            onClick={() => {
                                dispatch(openModView(userId));
                                onClose();
                            }}
                        >
                            <Icon
                                name='shield'
                                size='sm'
                            />
                        </button>
                    )}
                    <button
                        ref={moreButton}
                        type='button'
                        className={am('cover-btn')}
                        title={formatMessage({id: 'fusion.profile.more', defaultMessage: 'More'})}
                        aria-label={formatMessage({id: 'fusion.profile.moreFor', defaultMessage: 'Options for {name}'}, {name})}
                        aria-haspopup='menu'
                        aria-expanded={more}
                        onClick={() => setMore(!more)}
                    >
                        <Icon
                            name='dots'
                            size='sm'
                        />
                    </button>
                </div>
            </div>
            <div className={am('pbody')}>
                <Avatar
                    userId={userId}
                    size='xl'
                    status={true}
                />
                <h4 style={{display: 'flex', alignItems: 'center', gap: 6, color: roleColor}}>
                    {name}
                    {bot && <IntegrationTag agent={agent}/>}
                    <TeamTag user={user}/>
                </h4>
                {/* A busy agent's status starts with what it's doing; otherwise the plain status, as a person's. */}
                <div className={am('handle')}>
                    {agentBusy ? (
                        <>
                            {`@${user.username} · `}
                            <AgentActivityLine user={user}/>
                        </>
                    ) : `@${user.username} · ${formatMessage(STATUS_LABELS[status] || STATUS_LABELS.offline)}`}
                </div>
                {!bot && (
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
                        <ProfileFacts user={user}/>
                    </div>
                )}
                {bot && (
                    <div className={am('facts')}>
                        <span>
                            <Icon name='plug'/>
                            {user.bot_description || (agent ? formatMessage({id: 'fusion.profile.agent', defaultMessage: 'Agent · mention it or message it, like a coworker'}) : formatMessage({id: 'fusion.profile.bot', defaultMessage: 'Integration · posts on behalf of an app'}))}
                        </span>
                    </div>
                )}
                {timeout && (
                    <p
                        className={am('note', 'timed-out')}
                        role='status'
                    >
                        <Icon
                            name='timer'
                            size='sm'
                        />
                        {formatMessage({id: 'fusion.profile.timedOut', defaultMessage: 'Timed out until {time}, {date}'}, {time: formatTime(timeout.until), date: formatDate(timeout.until, {month: 'short', day: 'numeric'})})}
                    </p>
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
                {!isMe && !bot && <UserNote userId={userId}/>}
                <div className={am('plugin-attrs')}>
                    <Pluggable
                        pluggableName='PopoverUserAttributes'
                        user={user}
                        hide={onClose}
                        status={status}
                    />
                </div>
                <div
                    className={canCall ? am('actions2') : undefined}
                    style={{marginTop: 12}}
                >
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
                                {formatMessage({id: 'fusion.profile.editProfile', defaultMessage: 'Edit profile'})}
                            </button>
                            <button
                                ref={statusButton}
                                className={am('btn')}
                                onClick={() => setSettingStatus(true)}
                            >
                                <Icon
                                    name='smile'
                                    size='sm'
                                />
                                {formatMessage({id: 'fusion.profile.setStatus', defaultMessage: 'Set a status'})}
                            </button>
                            {systemConsole && (
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
                            style={{width: canCall ? undefined : '100%', justifyContent: 'center'}}
                            onClick={message}
                        >
                            <Icon
                                name='chat'
                                size='sm'
                            />
                            {formatMessage({id: 'fusion.profile.message', defaultMessage: 'Message'})}
                        </button>
                    )}
                    {canCall && (
                        <CallUserButton
                            userId={userId}
                            onDone={onClose}
                        />
                    )}
                </div>
                <div className={am('plugin-actions')}>
                    <Pluggable
                        pluggableName='PopoverUserActions'
                        user={user}
                        hide={onClose}
                        status={status}
                    />
                </div>
                {more && moreButton.current && (
                    <UserMenu
                        target={{userId,
                            anchor: moreButton.current,
                            point: (() => {
                                const rect = moreButton.current.getBoundingClientRect();
                                return {x: rect.right, y: rect.bottom + 4};
                            })()}}
                        onClose={() => {
                            setMore(false);
                            onClose();
                        }}
                    />
                )}
                {settingStatus && (
                    <StatusPopover
                        anchor={statusButton.current}
                        onClose={() => {
                            setSettingStatus(false);
                            onClose();
                        }}
                    />
                )}
            </div>
        </Popover>
    );
}
