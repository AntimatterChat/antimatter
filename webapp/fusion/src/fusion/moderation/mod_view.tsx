// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useState} from 'react';
import {defineMessage, useIntl} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {MemberModeration} from '@mattermost/types/moderation';

import {addUsersToGroup, removeUsersFromGroup} from 'mattermost-redux/actions/groups';
import {getRolesByNames} from 'mattermost-redux/actions/roles';
import {getTeamMember} from 'mattermost-redux/actions/teams';
import {updateUserActive} from 'mattermost-redux/actions/users';
import {Client4} from 'mattermost-redux/client';
import {Permissions} from 'mattermost-redux/constants';
import {getMemberTimeout, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam, getTeamMember as selectTeamMember} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUser, getCurrentUserId} from 'mattermost-redux/selectors/entities/users';
import {isSystemAdmin} from 'mattermost-redux/utils/user_utils';

import {openDirectChannelToUserId} from 'actions/channel_actions';
import {closeRightHandSide, showSearchResults, updateSearchTerms} from 'actions/views/rhs';
import {getModViewUserId} from 'selectors/rhs';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {useDisplayName, useUser, useUserStatus} from 'fusion/hooks/users';
import {STATUS_LABELS} from 'fusion/popovers/user_popover';
import {useDialogs} from 'fusion/shell/dialogs_context';
import {useToast} from 'fusion/shell/toast_context';
import {getMemberRolesForCurrentTeam, getTeamRoles, useRoleColor} from 'fusion/teams/roles';
import {TeamTag} from 'fusion/teams/team_card';
import {loadTeams} from 'fusion/teams/teams';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

import {canModerateTeam, timeOutMember} from './moderation';

// The permissions worth a moderator's glance, as Discord's mod view lists them.
const NOTABLE: Array<{permission: string; label: MessageDescriptor}> = [
    {permission: Permissions.MANAGE_TEAM, label: defineMessage({id: 'fusion.modView.perm.team', defaultMessage: 'Manage the team'})},
    {permission: Permissions.MANAGE_TEAM_ROLES, label: defineMessage({id: 'fusion.modView.perm.roles', defaultMessage: 'Manage roles'})},
    {permission: Permissions.REMOVE_USER_FROM_TEAM, label: defineMessage({id: 'fusion.modView.perm.remove', defaultMessage: 'Remove and time out members'})},
    {permission: Permissions.DELETE_OTHERS_POSTS, label: defineMessage({id: 'fusion.modView.perm.messages', defaultMessage: 'Manage messages'})},
    {permission: Permissions.MANAGE_PUBLIC_CHANNEL_PROPERTIES, label: defineMessage({id: 'fusion.modView.perm.channels', defaultMessage: 'Manage channels'})},
    {permission: Permissions.MANAGE_PUBLIC_CHANNEL_MEMBERS, label: defineMessage({id: 'fusion.modView.perm.members', defaultMessage: 'Manage channel members'})},
    {permission: Permissions.USE_CHANNEL_MENTIONS, label: defineMessage({id: 'fusion.modView.perm.everyone', defaultMessage: 'Mention @channel and @all'})},
];

// ModViewPanel is someone's mod view in the right-hand side, as Discord's: what a team's moderators look at before
// acting, and their actions, each offered to those allowed to.
export default function ModViewPanel() {
    const {formatMessage, formatDate, formatTime} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const dialogs = useDialogs();
    const userId = useSelector(getModViewUserId);
    const user = useUser(userId);
    const name = useDisplayName(user);
    const status = useUserStatus(userId);
    const roleColor = useRoleColor(userId);
    const me = useSelector(getCurrentUserId);
    const team = useSelector(getCurrentTeam);
    const teamId = team?.id || '';
    const [info, setInfo] = useState<MemberModeration | null>(null);
    const [infoFailed, setInfoFailed] = useState(false);

    const canModerate = useSelector((state: GlobalState) => canModerateTeam(state, teamId));
    const canRemove = useSelector((state: GlobalState) => haveITeamPermission(state, teamId, Permissions.REMOVE_USER_FROM_TEAM));
    const canManageRoles = useSelector((state: GlobalState) => haveITeamPermission(state, teamId, Permissions.MANAGE_TEAM_ROLES));
    const systemAdmin = useSelector((state: GlobalState) => isSystemAdmin(getCurrentUser(state)?.roles || ''));
    const timeout = useSelector((state: GlobalState) => getMemberTimeout(state, teamId, userId));
    const member = useSelector((state: GlobalState) => selectTeamMember(state, teamId, userId));
    const teamRoles = useSelector((state: GlobalState) => getTeamRoles(state, teamId));
    const theirRoles = useSelector((state: GlobalState) => getMemberRolesForCurrentTeam(state)[userId]);
    const roleNames = useMemo(() => [...(member?.roles || '').split(' '), ...(user?.roles || '').split(' ')].filter(Boolean), [member?.roles, user?.roles]);
    const permissions = useSelector((state: GlobalState) => {
        const all = new Set<string>();
        for (const n of roleNames) {
            for (const p of state.entities.roles.roles[n]?.permissions || []) {
                all.add(p);
            }
        }
        return [...all].sort().join(' ');
    });

    useEffect(() => {
        if (!teamId || !userId || !canModerate) {
            return undefined;
        }
        let gone = false;
        setInfo(null);
        setInfoFailed(false);
        dispatch(getTeamMember(teamId, userId));
        Client4.getMemberModeration(teamId, userId).
            then((m) => !gone && setInfo(m)).
            catch(() => !gone && setInfoFailed(true));
        return () => {
            gone = true;
        };
    }, [teamId, userId, canModerate, timeout?.until, dispatch]);

    useEffect(() => {
        if (roleNames.length) {
            dispatch(getRolesByNames(roleNames));
        }
    }, [roleNames, dispatch]);

    if (!user || !team || !canModerate) {
        return null;
    }
    const isMe = userId === me;
    const theyAreAdmin = isSystemAdmin(user.roles || '') || Boolean(member?.scheme_admin) || permissions.split(' ').includes(Permissions.MANAGE_TEAM);
    const actionable = !isMe && !theyAreAdmin;
    const notable = NOTABLE.filter((n) => isSystemAdmin(user.roles || '') || permissions.split(' ').includes(n.permission));
    const missingRoles = teamRoles.filter((r) => !(theirRoles || []).some((t) => t.id === r.id));

    const fail = (error: unknown, fallback: string) => toast((error as {message?: string})?.message || fallback);
    const message = async () => {
        const result = await dispatch(openDirectChannelToUserId(userId));
        if ('data' in result && result.data) {
            getHistory().push(channelPath(team.name, result.data));
        }
    };
    const theirMessages = () => {
        dispatch(updateSearchTerms(`from:${user.username} `));
        dispatch(showSearchResults());
    };
    const liftTimeout = async () => {
        const result = await dispatch(timeOutMember(teamId, userId, 0));
        if ('error' in result && result.error) {
            fail(result.error, formatMessage({id: 'fusion.timeout.liftFailed', defaultMessage: 'The timeout couldn\'t be lifted.'}));
        }
    };
    const setRole = async (roleId: string, on: boolean) => {
        const result = await dispatch(on ? addUsersToGroup(roleId, [userId]) : removeUsersFromGroup(roleId, [userId]));
        if ('error' in result && result.error) {
            fail(result.error, formatMessage({id: 'fusion.modView.roleFailed', defaultMessage: 'The role couldn\'t be changed.'}));
            return;
        }
        dispatch(loadTeams());
    };
    const deactivate = () => dialogs.confirm({
        title: formatMessage({id: 'fusion.modView.deactivateTitle', defaultMessage: 'Deactivate {name}?'}, {name}),
        message: formatMessage({id: 'fusion.modView.deactivateMessage', defaultMessage: 'They\'re signed out everywhere and can\'t sign in again until a system admin reactivates their account. Their messages stay.'}),
        confirmLabel: formatMessage({id: 'fusion.modView.deactivate', defaultMessage: 'Deactivate account'}),
        danger: true,
        onConfirm: async () => {
            const result = await dispatch(updateUserActive(userId, false));
            if ('error' in result && result.error) {
                fail(result.error, formatMessage({id: 'fusion.modView.deactivateFailed', defaultMessage: 'The account couldn\'t be deactivated.'}));
            }
        },
    });

    const date = (at: number) => formatDate(at, {year: 'numeric', month: 'short', day: 'numeric'});
    return (
        <div
            className={am('mod-view')}
            aria-label={formatMessage({id: 'fusion.modView.title', defaultMessage: 'Mod view'})}
        >
            <div className={am('rhs-head')}>
                <h3>
                    <Icon name='shield'/>
                    {formatMessage({id: 'fusion.modView.title', defaultMessage: 'Mod view'})}
                </h3>
                <span className={am('grow')}/>
                <button
                    type='button'
                    className={am('icon-btn')}
                    aria-label={formatMessage({id: 'fusion.modView.close', defaultMessage: 'Close the mod view'})}
                    onClick={() => dispatch(closeRightHandSide())}
                >
                    <Icon name='x'/>
                </button>
            </div>
            <div className={am('mod-body')}>
                <div className={am('mod-who')}>
                    <Avatar
                        userId={userId}
                        size='lg'
                        status={true}
                    />
                    <div>
                        <b style={roleColor ? {color: roleColor} : undefined}>
                            {name}
                            <TeamTag user={user}/>
                        </b>
                        <span>{`@${user.username} · ${formatMessage(STATUS_LABELS[status] || STATUS_LABELS.offline)}`}</span>
                    </div>
                </div>

                {timeout && (
                    <div
                        className={am('mod-timeout')}
                        role='status'
                    >
                        <Icon name='timer'/>
                        <div className={am('grow')}>
                            <b>{formatMessage({id: 'fusion.modView.timedOut', defaultMessage: 'Timed out until {time}, {date}'}, {time: formatTime(timeout.until), date: date(timeout.until)})}</b>
                            {timeout.reason && <span>{timeout.reason}</span>}
                        </div>
                        {canRemove && actionable && (
                            <button
                                type='button'
                                className={am('btn', 'small')}
                                onClick={liftTimeout}
                            >
                                {formatMessage({id: 'fusion.modView.lift', defaultMessage: 'Remove timeout'})}
                            </button>
                        )}
                    </div>
                )}

                <h3 className={am('section-title')}>{formatMessage({id: 'fusion.modView.summary', defaultMessage: 'Summary'})}</h3>
                <dl className={am('mod-facts')}>
                    <dt>{formatMessage({id: 'fusion.modView.joined', defaultMessage: 'Member since'})}</dt>
                    <dd>{info ? date(info.joined_at) : '…'}</dd>
                    <dt>{formatMessage({id: 'fusion.modView.created', defaultMessage: 'Account created'})}</dt>
                    <dd>{date(user.create_at)}</dd>
                    <dt>{formatMessage({id: 'fusion.modView.messages', defaultMessage: 'Messages in the team'})}</dt>
                    <dd>
                        {info ? formatMessage({id: 'fusion.modView.messageCounts', defaultMessage: '{day} in the last day · {week} in the last week'}, {day: info.posts_last_day, week: info.posts_last_week}) : '…'}
                        <button
                            type='button'
                            className={am('link')}
                            onClick={theirMessages}
                        >
                            {formatMessage({id: 'fusion.modView.seeMessages', defaultMessage: 'See them'})}
                        </button>
                    </dd>
                    {user.email && (
                        <>
                            <dt>{formatMessage({id: 'fusion.modView.email', defaultMessage: 'Email'})}</dt>
                            <dd>{user.email}</dd>
                        </>
                    )}
                </dl>
                {infoFailed && <p className={am('lead')}>{formatMessage({id: 'fusion.modView.infoFailed', defaultMessage: 'Some of their details couldn\'t be loaded.'})}</p>}

                <h3 className={am('section-title')}>{formatMessage({id: 'fusion.modView.roles', defaultMessage: 'Roles'})}</h3>
                <div className={am('mod-roles')}>
                    {(theirRoles || []).map((r) => (
                        <span
                            key={r.id}
                            className={am('role-chip')}
                        >
                            <i
                                className={am('role-dot')}
                                style={{background: r.color || 'var(--am-muted)'}}
                            />
                            {r.display_name}
                            {canManageRoles && (
                                <button
                                    type='button'
                                    aria-label={formatMessage({id: 'fusion.modView.removeRole', defaultMessage: 'Remove the role {role}'}, {role: r.display_name})}
                                    onClick={() => setRole(r.id, false)}
                                >
                                    <Icon
                                        name='x'
                                        size='xs'
                                    />
                                </button>
                            )}
                        </span>
                    ))}
                    {!theirRoles?.length && <span className={am('muted')}>{formatMessage({id: 'fusion.modView.noRoles', defaultMessage: 'No roles'})}</span>}
                    {canManageRoles && missingRoles.length > 0 && (
                        <select
                            value=''
                            aria-label={formatMessage({id: 'fusion.modView.addRole', defaultMessage: 'Add a role'})}
                            onChange={(e) => e.target.value && setRole(e.target.value, true)}
                        >
                            <option value=''>{formatMessage({id: 'fusion.modView.addRole', defaultMessage: 'Add a role'})}</option>
                            {missingRoles.map((r) => (
                                <option
                                    key={r.id}
                                    value={r.id}
                                >
                                    {r.display_name}
                                </option>
                            ))}
                        </select>
                    )}
                </div>

                <h3 className={am('section-title')}>{formatMessage({id: 'fusion.modView.permissions', defaultMessage: 'Notable permissions'})}</h3>
                <ul className={am('mod-perms')}>
                    {isSystemAdmin(user.roles || '') && <li><b>{formatMessage({id: 'fusion.modView.systemAdmin', defaultMessage: 'System admin: every permission'})}</b></li>}
                    {notable.map((n) => <li key={n.permission}>{formatMessage(n.label)}</li>)}
                    {!notable.length && <li className={am('muted')}>{formatMessage({id: 'fusion.modView.noPerms', defaultMessage: 'Nothing beyond a member\'s'})}</li>}
                </ul>

                <h3 className={am('section-title')}>{formatMessage({id: 'fusion.modView.actions', defaultMessage: 'Actions'})}</h3>
                <div className={am('mod-actions')}>
                    {!isMe && (
                        <button
                            type='button'
                            className={am('btn')}
                            onClick={message}
                        >
                            <Icon
                                name='chat'
                                size='sm'
                            />
                            {formatMessage({id: 'fusion.modView.message', defaultMessage: 'Message'})}
                        </button>
                    )}
                    {canRemove && actionable && !timeout && (
                        <button
                            type='button'
                            className={am('btn')}
                            onClick={() => dialogs.timeOut(userId)}
                        >
                            <Icon
                                name='timer'
                                size='sm'
                            />
                            {formatMessage({id: 'fusion.modView.timeOut', defaultMessage: 'Time out'})}
                        </button>
                    )}
                    {canRemove && actionable && (
                        <button
                            type='button'
                            className={am('btn', 'danger-ghost')}
                            onClick={() => dialogs.removeFromTeam(userId)}
                        >
                            <Icon
                                name='leave'
                                size='sm'
                            />
                            {formatMessage({id: 'fusion.modView.remove', defaultMessage: 'Remove from team'})}
                        </button>
                    )}
                    {systemAdmin && !isMe && !isSystemAdmin(user.roles || '') && user.delete_at === 0 && (
                        <button
                            type='button'
                            className={am('btn', 'danger-ghost')}
                            onClick={deactivate}
                        >
                            <Icon
                                name='x'
                                size='sm'
                            />
                            {formatMessage({id: 'fusion.modView.deactivate', defaultMessage: 'Deactivate account'})}
                        </button>
                    )}
                </div>
                {!actionable && !isMe && <p className={am('lead')}>{formatMessage({id: 'fusion.modView.adminNote', defaultMessage: 'Admins can\'t be timed out or removed here.'})}</p>}
            </div>
        </div>
    );
}
