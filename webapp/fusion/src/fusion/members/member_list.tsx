// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {UserProfile} from '@mattermost/types/users';

import {countPendingChannelJoinRequests, getChannelJoinRequests, getChannelStats} from 'mattermost-redux/actions/channels';
import {canManageChannelJoinRequests, getAllChannelStats, getCurrentChannelId, getCurrentChannel, getPendingChannelJoinRequests} from 'mattermost-redux/selectors/entities/channels';
import {isTimedOut} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeamId, getTeam} from 'mattermost-redux/selectors/entities/teams';
import {makeGetProfilesInChannel} from 'mattermost-redux/selectors/entities/users';
import {isGuest} from 'mattermost-redux/utils/user_utils';

import {loadProfilesAndReloadChannelMembers} from 'actions/user_actions';
import {closeRightHandSide} from 'actions/views/rhs';
import {makeGetCustomStatus} from 'selectors/views/custom_status';

import PendingJoinRequests from 'components/channel_members_rhs/pending_join_requests';

import {AgentActivityLine, IntegrationTag, useAgentBusy} from 'fusion/agents/agent_status';
import {useMemberListAttribute} from 'fusion/attributes/member_list_attribute';
import {TalkNote} from 'fusion/calls/markers';
import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {SheetBack} from 'fusion/components/layer';
import StatusEmoji from 'fusion/components/status_emoji';
import UserTag from 'fusion/components/user_tag';
import {CATEGORY_FUSION_DISPLAY, MEMBER_GROUPING} from 'fusion/hooks/display';
import {useIsPhone} from 'fusion/hooks/media';
import {useDisplayName} from 'fusion/hooks/users';
import {usePreference} from 'fusion/modals/settings_controls';
import {useUserMenu} from 'fusion/popovers/user_menu';
import UserPopover from 'fusion/popovers/user_popover';
import {useLayout} from 'fusion/shell/layout_context';
import {useRoleColor} from 'fusion/teams/roles';
import {TeamTag} from 'fusion/teams/team_card';
import {getTeams, useRoleGroupsAllowed} from 'fusion/teams/teams';
import {am} from 'fusion/utils/class_names';
import {openAddToGroup} from 'fusion/utils/modals';

import type {GlobalState} from 'types/store';

import MemberAddPopover, {useCanAddPeople} from './member_add_popover';
import {ROLE_COLORS, memberGrouping, useMemberGroups, useMemberRoles} from './member_groups';
import type {Grouping, MemberRole} from './member_groups';

const PER_PAGE = 100;

// matchesSearch tells whether a typed term matches a member's name, as the member search box filters the list:
// their username, nickname or full name, case-insensitively.
export function matchesSearch(user: UserProfile, term: string): boolean {
    const needle = term.trim().toLowerCase();
    if (!needle) {
        return true;
    }
    return [user.username, user.nickname, user.first_name, user.last_name, `${user.first_name} ${user.last_name}`].
        some((value) => Boolean(value) && value.toLowerCase().includes(needle));
}

type MemberProps = {
    user: UserProfile;
    off: boolean;
    role: MemberRole;

    // The community attribute shown in the member list, as Company.
    attribute?: string;
    onOpen: (el: HTMLElement) => void;
    onMenu: (e: React.MouseEvent<HTMLElement>) => void;
};

const NO_REQUESTS: ReturnType<typeof getPendingChannelJoinRequests> = [];

// Member is one row of the list (exported for its tests).
export function Member({user, off, role, attribute, onOpen, onMenu}: MemberProps) {
    const {formatMessage} = useIntl();
    const name = useDisplayName(user);
    const roleColor = useRoleColor(user.id);
    const getCustomStatus = useMemo(() => makeGetCustomStatus(), []);
    const custom = useSelector((state: GlobalState) => getCustomStatus(state, user.id));
    const timedOut = useSelector((state: GlobalState) => isTimedOut(state, getCurrentTeamId(state), user.id));
    const agentBusy = useAgentBusy(user);
    return (
        <button
            className={am('member', {off})}
            onClick={(e) => onOpen(e.currentTarget)}
            onContextMenu={onMenu}
        >
            <Avatar
                userId={user.id}
                status={true}
            />
            <span className={am('who')}>
                <b style={roleColor || role ? {color: roleColor || ROLE_COLORS[role]} : undefined}>
                    <span className={am('nm')}>{name}</span>
                    {user.remote_id && (
                        <span
                            className={am('remote-tag')}
                            title={formatMessage({id: 'fusion.members.remote', defaultMessage: 'From another server'})}
                        >
                            <Icon
                                name='globe'
                                size='xs'
                            />
                        </span>
                    )}
                    <StatusEmoji userId={user.id}/>
                    {timedOut && (
                        <span
                            className={am('timeout-tag')}
                            title={formatMessage({id: 'fusion.members.timedOut', defaultMessage: 'Timed out'})}
                        >
                            <Icon
                                name='timer'
                                size='xs'
                            />
                        </span>
                    )}
                    {user.is_bot && <IntegrationTag agent={Boolean(user.is_agent)}/>}
                    <TeamTag
                        user={user}
                        label={true}
                    />
                    {isGuest(user.roles) && <UserTag kind='guest'/>}
                </b>
                <span>
                    {/* A busy agent's status line starts with its activity; otherwise its plain status, as a person's. */}
                    {agentBusy ? <AgentActivityLine user={user}/> : <TalkNote userId={user.id}>{custom?.text || attribute || user.position || ''}</TalkNote>}
                </span>
            </span>
        </button>
    );
}

// MemberList lists the conversation's members, online first, like the mockup's member column.
export default function MemberList({inPanel = false}: {inPanel?: boolean}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const getProfilesInChannel = useMemo(() => makeGetProfilesInChannel(), []);
    const channelId = useSelector(getCurrentChannelId);
    const channel = useSelector(getCurrentChannel);
    const profiles = useSelector((state: GlobalState) => getProfilesInChannel(state, channelId, {active: true}));
    const memberCount = useSelector((state: GlobalState) => getAllChannelStats(state)[channelId]?.member_count);
    const roles = useMemberRoles(profiles, channelId);

    // People are listed by role (when the server's admins allow it) or by team: as the community's admins chose, or as
    // each person chooses when they let them.
    const rolesAllowed = useRoleGroupsAllowed();
    const hasTeams = useSelector((state: GlobalState) => getTeams(state).length > 0);
    const [chosenGrouping, setGrouping] = usePreference(CATEGORY_FUSION_DISPLAY, MEMBER_GROUPING, 'roles');
    const teamId = useSelector(getCurrentTeamId);
    const view = useSelector((state: GlobalState) => getTeam(state, teamId)?.member_list_view);
    const grouping = memberGrouping(view, chosenGrouping, hasTeams, rolesAllowed);
    const membersChoose = !view || view === 'choice';
    const listAttribute = useMemberListAttribute(channel?.type === 'D' || channel?.type === 'G' ? undefined : teamId, profiles);
    const shownProfiles = listAttribute.filtering ? profiles.filter((u) => listAttribute.shown(u.id)) : profiles;
    const [search, setSearch] = useState('');
    const term = search.trim();
    const searchedProfiles = term ? shownProfiles.filter((u) => matchesSearch(u, term)) : shownProfiles;
    const groups = useMemberGroups(searchedProfiles, roles, grouping);
    const canAdd = useCanAddPeople(channel);
    const [popover, setPopover] = useState<{userId: string; anchor: HTMLElement} | null>(null);
    const [adding, setAdding] = useState(false);
    const page = useRef(0);
    const loading = useRef(false);
    const [userMenu, openUserMenu, closeUserMenu] = useUserMenu();
    const addRef = useRef<HTMLButtonElement>(null);
    const layout = useLayout();
    const phone = useIsPhone();

    useEffect(() => {
        if (channelId) {
            page.current = 0;
            dispatch(loadProfilesAndReloadChannelMembers(0, PER_PAGE, channelId));
            dispatch(getChannelStats(channelId));
        }
    }, [channelId, dispatch]);

    // Requests to join a discoverable private channel, for those who can let people in.
    const canManageRequests = useSelector((state: GlobalState) => canManageChannelJoinRequests(state, channel));
    const joinRequests = useSelector((state: GlobalState) => (canManageRequests && channelId ? getPendingChannelJoinRequests(state, channelId) : NO_REQUESTS));
    useEffect(() => {
        if (canManageRequests && channelId) {
            dispatch(getChannelJoinRequests(channelId, {status: 'pending'}));
            dispatch(countPendingChannelJoinRequests(channelId));
        }
    }, [canManageRequests, channelId, dispatch]);

    const direct = channel?.type === 'D' || channel?.type === 'G';

    const list = (
        <>
            <div className={am('mem-head')}>
                {/* On a phone the list takes the whole screen: back to the conversation. */}
                {phone && <SheetBack onBack={() => (inPanel ? dispatch(closeRightHandSide()) : layout.setRightOpen(false))}/>}
                <span>{formatMessage({id: 'fusion.members.title', defaultMessage: '{count, plural, one {# member} other {# members}}'}, {count: Math.max(memberCount ?? 0, profiles.length)})}</span>
                {rolesAllowed && hasTeams && !direct && membersChoose && (
                    <span
                        className={am('mem-grouping')}
                        role='radiogroup'
                        aria-label={formatMessage({id: 'fusion.members.grouping', defaultMessage: 'Group members by'})}
                    >
                        {(['roles', 'teams'] as Grouping[]).map((g) => {
                            const label = g === 'roles' ? formatMessage({id: 'fusion.members.byRole', defaultMessage: 'Roles'}) : formatMessage({id: 'fusion.members.byTeam', defaultMessage: 'Teams'});
                            return (
                                <button
                                    key={g}
                                    type='button'
                                    role='radio'
                                    aria-checked={grouping === g}
                                    aria-label={label}
                                    title={label}
                                    className={am('icon-btn', {on: grouping === g})}
                                    onClick={() => setGrouping(g)}
                                >
                                    <Icon
                                        name={g === 'roles' ? 'rank' : 'org'}
                                        size='sm'
                                    />
                                </button>
                            );
                        })}
                    </span>
                )}
                {listAttribute.filter}
                {!direct && (canAdd.add || canAdd.invite) && (
                    <button
                        ref={addRef}
                        className={am('icon-btn', 'mem-add')}
                        title={formatMessage({id: 'fusion.members.add', defaultMessage: 'Add people or invite'})}
                        aria-label={formatMessage({id: 'fusion.members.addLabel', defaultMessage: 'Add people or create an invite'})}
                        aria-haspopup='menu'
                        aria-expanded={adding}
                        onClick={() => {
                            if (channel?.type === 'G') {
                                dispatch(openAddToGroup());
                            } else {
                                setAdding(!adding);
                            }
                        }}
                    >
                        <Icon
                            name='plus'
                            size='sm'
                        />
                    </button>
                )}
                {inPanel && !phone && (
                    <button
                        className={am('icon-btn', 'mem-add')}
                        aria-label={formatMessage({id: 'fusion.rhs.close', defaultMessage: 'Close'})}
                        onClick={() => dispatch(closeRightHandSide())}
                    >
                        <Icon
                            name='x'
                            size='sm'
                        />
                    </button>
                )}
            </div>
            {!direct && (
                <label className={am('dm-find', 'mem-search')}>
                    <Icon
                        name='search'
                        size='sm'
                    />
                    <input
                        type='search'
                        value={search}
                        placeholder={formatMessage({id: 'fusion.members.search', defaultMessage: 'Search members'})}
                        aria-label={formatMessage({id: 'fusion.members.search', defaultMessage: 'Search members'})}
                        autoComplete='off'
                        onChange={(e) => setSearch(e.target.value)}
                    />
                </label>
            )}
            {canManageRequests && joinRequests.length > 0 && (
                <div className={am('join-requests')}>
                    <PendingJoinRequests
                        channelId={channelId}
                        requests={joinRequests}
                    />
                </div>
            )}
            {groups.map((group) => (
                <React.Fragment key={group.key}>
                    <div className={am('role-head')}>
                        {group.color && (
                            <span
                                className={am('sw')}
                                style={{background: group.color}}
                            />
                        )}
                        {group.tag && <span className={am('team-tag', 'static')}>{group.tag}</span>}
                        {group.label}
                    </div>
                    {group.users.map((u) => (
                        <Member
                            key={u.id}
                            user={u}
                            off={group.off || Boolean(group.offline?.has(u.id))}
                            role={roles[u.id]}
                            attribute={listAttribute.textOf(u.id)}
                            onOpen={(anchor) => {
                                // A click while their menu is open only closes it.
                                if (userMenu) {
                                    closeUserMenu();
                                    return;
                                }
                                setPopover((open) => (open?.userId === u.id ? null : {userId: u.id, anchor}));
                            }}
                            onMenu={(e) => openUserMenu(u.id, e)}
                        />
                    ))}
                </React.Fragment>
            ))}
            {term && searchedProfiles.length === 0 && (
                <p className={am('pick-empty')}>{formatMessage({id: 'fusion.members.noMatch', defaultMessage: 'No member matches.'})}</p>
            )}
            {popover && (
                <UserPopover
                    userId={popover.userId}
                    anchor={popover.anchor}
                    onClose={() => setPopover(null)}
                />
            )}
            {adding && channel && (
                <MemberAddPopover
                    channel={channel}
                    anchor={addRef.current}
                    onClose={() => setAdding(false)}
                />
            )}
            {userMenu}
        </>
    );

    // Big channels load their members a page at a time, as the list nears its end.
    const onScroll = async (e: React.UIEvent<HTMLElement>) => {
        const el = e.currentTarget;
        if (loading.current || !memberCount || profiles.length >= memberCount || el.scrollHeight - el.scrollTop - el.clientHeight > 400) {
            return;
        }
        loading.current = true;
        page.current += 1;
        await dispatch(loadProfilesAndReloadChannelMembers(page.current, PER_PAGE, channelId));
        loading.current = false;
    };

    if (inPanel) {
        return (
            <div
                className={am('rhs-body', 'members-panel')}
                onScroll={onScroll}
            >
                {list}
            </div>
        );
    }
    return (
        <aside
            className={am('members')}
            onScroll={onScroll}
            aria-label={formatMessage({id: 'fusion.members.label', defaultMessage: 'Members'})}
        >
            {list}
        </aside>
    );
}
