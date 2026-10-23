// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useId, useMemo, useRef, useState} from 'react';
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
import type {IconName} from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {MenuItem, MenuKeys, rovingKeyDown} from 'fusion/components/menu';
import {useDisplayName, useUser, useUserStatus} from 'fusion/hooks/users';
import {STATUS_LABELS} from 'fusion/popovers/user_popover';
import {useCommunityProfile} from 'fusion/profiles/community_profiles';
import {useDialogs} from 'fusion/shell/dialogs_context';
import {useToast} from 'fusion/shell/toast_context';
import {getGivenRolesOf, getGivenTeamRoles, useRoleColor} from 'fusion/teams/roles';
import {TeamTag} from 'fusion/teams/team_card';
import {loadTeams} from 'fusion/teams/teams';
import {am} from 'fusion/utils/class_names';
import {channelPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

import {canModerateTeam, timeOutMember} from './moderation';

// The permissions worth a moderator's glance, as Discord's mod view lists them.
const NOTABLE: Array<{permission: string; label: MessageDescriptor}> = [
    {permission: Permissions.MANAGE_TEAM, label: defineMessage({id: 'fusion.modView.perm.team', defaultMessage: 'Manage the community'})},
    {permission: Permissions.MANAGE_TEAM_ROLES, label: defineMessage({id: 'fusion.modView.perm.roles', defaultMessage: 'Manage roles'})},
    {permission: Permissions.REMOVE_USER_FROM_TEAM, label: defineMessage({id: 'fusion.modView.perm.remove', defaultMessage: 'Remove and time out members'})},
    {permission: Permissions.DELETE_OTHERS_POSTS, label: defineMessage({id: 'fusion.modView.perm.messages', defaultMessage: 'Manage messages'})},
    {permission: Permissions.MANAGE_PUBLIC_CHANNEL_PROPERTIES, label: defineMessage({id: 'fusion.modView.perm.channels', defaultMessage: 'Manage channels'})},
    {permission: Permissions.MANAGE_PUBLIC_CHANNEL_MEMBERS, label: defineMessage({id: 'fusion.modView.perm.members', defaultMessage: 'Manage channel members'})},
    {permission: Permissions.USE_CHANNEL_MENTIONS, label: defineMessage({id: 'fusion.modView.perm.everyone', defaultMessage: 'Mention @channel and @all'})},
];

// One of the mod view's actions: data-action names it, once in the panel.
type ModAction = {
    id: string;
    icon: IconName;
    label: string;
    danger?: boolean;
    run: () => void;
};

// ModViewPanel is someone's mod view in the right-hand side, as Discord's: who they are and their state (a timeout),
// then the actions a team's moderators take, each once and offered to those allowed to (the destructive ones last,
// each asking first), then what they look at before acting: a summary, their roles and their notable permissions.
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
    const [addingRole, setAddingRole] = useState(false);
    const addRoleRef = useRef<HTMLButtonElement>(null);
    const actionsRef = useRef<HTMLDivElement>(null);
    const titleId = useId();

    const canModerate = useSelector((state: GlobalState) => canModerateTeam(state, teamId));
    const canRemove = useSelector((state: GlobalState) => haveITeamPermission(state, teamId, Permissions.REMOVE_USER_FROM_TEAM));
    const canManageRoles = useSelector((state: GlobalState) => haveITeamPermission(state, teamId, Permissions.MANAGE_TEAM_ROLES));
    const systemAdmin = useSelector((state: GlobalState) => isSystemAdmin(getCurrentUser(state)?.roles || ''));
    const timeout = useSelector((state: GlobalState) => getMemberTimeout(state, teamId, userId));
    const [communityProfile] = useCommunityProfile(userId, teamId);
    const member = useSelector((state: GlobalState) => selectTeamMember(state, teamId, userId));

    // The roles given to some (not the default role, which everyone has).
    const teamRoles = useSelector((state: GlobalState) => getGivenTeamRoles(state, teamId));
    const theirRoles = useSelector((state: GlobalState) => getGivenRolesOf(state, userId));
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

    // A nickname or a picture for the community that breaks its rules: taken off (the person can set another).
    const resetProfile = async (reset: {nickname?: boolean; avatar?: boolean}, done: string, failed: string) => {
        try {
            await Client4.resetCommunityProfile(teamId, userId, reset);
            toast(done);
        } catch (error) {
            fail(error, failed);
        }
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

    // The actions, in the order a moderator reaches for them: talk to them, look at what they wrote, time them out
    // (or lift it); then, set apart, removing them from the community and deactivating their account.
    const actions: ModAction[] = [];
    const destructive: ModAction[] = [];
    if (!isMe) {
        actions.push({id: 'message', icon: 'chat', label: formatMessage({id: 'fusion.userMenu.message', defaultMessage: 'Message'}), run: message});
    }
    actions.push({id: 'search-messages', icon: 'search', label: formatMessage({id: 'fusion.modView.searchMessages', defaultMessage: 'Search their messages'}), run: theirMessages});
    if (canRemove && actionable) {
        actions.push(timeout ? {id: 'lift-timeout', icon: 'timer', label: formatMessage({id: 'fusion.userMenu.liftTimeout', defaultMessage: 'Remove timeout'}), run: liftTimeout} : {id: 'time-out', icon: 'timer', label: formatMessage({id: 'fusion.userMenu.timeOut', defaultMessage: 'Time out…'}), run: () => dialogs.timeOut(userId)});
        if (communityProfile?.nickname) {
            actions.push({id: 'reset-nickname', icon: 'undo', label: formatMessage({id: 'fusion.modView.resetNickname', defaultMessage: 'Reset nickname'}), run: () => resetProfile({nickname: true}, formatMessage({id: 'fusion.modView.nicknameReset', defaultMessage: 'Nickname reset'}), formatMessage({id: 'fusion.modView.resetFailed', defaultMessage: 'The profile couldn\'t be reset.'}))});
        }
        if (communityProfile?.avatar_update_at) {
            actions.push({id: 'reset-picture', icon: 'image', label: formatMessage({id: 'fusion.modView.resetPicture', defaultMessage: 'Reset picture'}), run: () => resetProfile({avatar: true}, formatMessage({id: 'fusion.modView.pictureReset', defaultMessage: 'Picture reset'}), formatMessage({id: 'fusion.modView.resetFailed', defaultMessage: 'The profile couldn\'t be reset.'}))});
        }
        destructive.push({id: 'remove-from-community', icon: 'leave', danger: true, label: formatMessage({id: 'fusion.userMenu.removeTeam', defaultMessage: 'Remove from community…'}), run: () => dialogs.removeFromTeam(userId)});
    }
    if (systemAdmin && !isMe && !isSystemAdmin(user.roles || '') && user.delete_at === 0) {
        destructive.push({id: 'deactivate', icon: 'ban', danger: true, label: formatMessage({id: 'fusion.modView.deactivateAsk', defaultMessage: 'Deactivate account…'}), run: deactivate});
    }
    const actionButton = (action: ModAction) => (
        <button
            key={action.id}
            type='button'
            data-action={action.id}
            className={am('mod-act', {danger: action.danger})}
            onClick={action.run}
        >
            <Icon
                name={action.icon}
                size='sm'
            />
            <span>{action.label}</span>
        </button>
    );

    return (
        <section
            className={am('mod-view')}
            aria-labelledby={titleId}
        >
            <div className={am('rhs-head')}>
                <h3 id={titleId}>
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
                            <span className={am('mod-name')}>{name}</span>
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
                    </div>
                )}

                {/* The actions are one toolbar: Tab reaches it, the arrow keys move along it. */}
                <div
                    ref={actionsRef}
                    className={am('mod-actions')}
                    role='toolbar'
                    aria-orientation='vertical'
                    aria-label={formatMessage({id: 'fusion.modView.actions', defaultMessage: 'Actions'})}
                    onKeyDown={(e) => rovingKeyDown(e, actionsRef.current, 'button')}
                >
                    {actions.map(actionButton)}
                    {destructive.length > 0 && (
                        <div
                            className={am('mod-sep')}
                            role='separator'
                        />
                    )}
                    {destructive.map(actionButton)}
                </div>
                {!actionable && !isMe && <p className={am('mod-note')}>{formatMessage({id: 'fusion.modView.adminNote', defaultMessage: 'Admins can\'t be timed out or removed here.'})}</p>}

                <h4 className={am('section-title')}>{formatMessage({id: 'fusion.modView.summary', defaultMessage: 'Summary'})}</h4>
                <dl className={am('mod-facts')}>
                    <dt>{formatMessage({id: 'fusion.modView.joined', defaultMessage: 'Member since'})}</dt>
                    <dd>{info ? date(info.joined_at) : '…'}</dd>
                    <dt>{formatMessage({id: 'fusion.modView.created', defaultMessage: 'Account created'})}</dt>
                    <dd>{date(user.create_at)}</dd>
                    <dt>{formatMessage({id: 'fusion.modView.messages', defaultMessage: 'Messages in the community'})}</dt>
                    <dd>{info ? formatMessage({id: 'fusion.modView.messageCounts', defaultMessage: '{day} in the last day · {week} in the last week'}, {day: info.posts_last_day, week: info.posts_last_week}) : '…'}</dd>
                    {user.email && (
                        <>
                            <dt>{formatMessage({id: 'fusion.modView.email', defaultMessage: 'Email'})}</dt>
                            <dd>{user.email}</dd>
                        </>
                    )}
                </dl>
                {infoFailed && <p className={am('mod-note')}>{formatMessage({id: 'fusion.modView.infoFailed', defaultMessage: 'Some of their details couldn\'t be loaded.'})}</p>}

                <h4 className={am('section-title')}>{formatMessage({id: 'fusion.modView.roles', defaultMessage: 'Roles'})}</h4>
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
                                    data-action={`remove-role:${r.id}`}
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
                        <button
                            ref={addRoleRef}
                            type='button'
                            data-action='add-role'
                            className={am('mod-add-role', {on: addingRole})}
                            aria-haspopup='menu'
                            aria-expanded={addingRole}
                            onClick={() => setAddingRole(!addingRole)}
                        >
                            <Icon
                                name='plus'
                                size='xs'
                            />
                            {formatMessage({id: 'fusion.modView.addRole', defaultMessage: 'Add a role'})}
                        </button>
                    )}
                </div>
                {addingRole && (
                    <Popover
                        anchor={addRoleRef.current}
                        placement='below-start'
                        className='plus-pop ch-menu mod-role-menu'
                        role='menu'
                        label={formatMessage({id: 'fusion.modView.addRole', defaultMessage: 'Add a role'})}
                        onClose={() => setAddingRole(false)}
                    >
                        <MenuKeys>
                            {missingRoles.map((r) => (
                                <MenuItem
                                    key={r.id}
                                    action={`add-role:${r.id}`}
                                    glyph={(
                                        <i
                                            className={am('role-dot')}
                                            style={{background: r.color || 'var(--am-muted)'}}
                                        />
                                    )}
                                    label={r.display_name}
                                    onClick={() => {
                                        setAddingRole(false);
                                        setRole(r.id, true);
                                    }}
                                />
                            ))}
                        </MenuKeys>
                    </Popover>
                )}

                <h4 className={am('section-title')}>{formatMessage({id: 'fusion.modView.permissions', defaultMessage: 'Notable permissions'})}</h4>
                <ul className={am('mod-perms')}>
                    {isSystemAdmin(user.roles || '') && <li><b>{formatMessage({id: 'fusion.modView.systemAdmin', defaultMessage: 'System admin: every permission'})}</b></li>}
                    {notable.map((n) => <li key={n.permission}>{formatMessage(n.label)}</li>)}
                    {!notable.length && <li className={am('muted')}>{formatMessage({id: 'fusion.modView.noPerms', defaultMessage: 'No permissions beyond a member\'s'})}</li>}
                </ul>
            </div>
        </section>
    );
}
