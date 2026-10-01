// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {UserProfile} from '@mattermost/types/users';

import {getChannelStats} from 'mattermost-redux/actions/channels';
import {getAllChannelStats, getCurrentChannelId, getCurrentChannel} from 'mattermost-redux/selectors/entities/channels';
import {makeGetProfilesInChannel} from 'mattermost-redux/selectors/entities/users';

import {loadProfilesAndReloadChannelMembers} from 'actions/user_actions';
import {closeRightHandSide} from 'actions/views/rhs';
import {makeGetCustomStatus} from 'selectors/views/custom_status';

import {TalkNote} from 'fusion/calls/markers';
import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {useDisplayName} from 'fusion/hooks/users';
import {useUserMenu} from 'fusion/popovers/user_menu';
import UserPopover from 'fusion/popovers/user_popover';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import MemberAddPopover, {useCanAddPeople} from './member_add_popover';
import {ROLE_COLORS, useMemberGroups, useMemberRoles} from './member_groups';
import type {MemberRole} from './member_groups';

const PER_PAGE = 100;

type MemberProps = {
    user: UserProfile;
    off: boolean;
    role: MemberRole;
    onOpen: (el: HTMLElement) => void;
    onMenu: (e: React.MouseEvent<HTMLElement>) => void;
};

function Member({user, off, role, onOpen, onMenu}: MemberProps) {
    const name = useDisplayName(user);
    const getCustomStatus = useMemo(() => makeGetCustomStatus(), []);
    const custom = useSelector((state: GlobalState) => getCustomStatus(state, user.id));
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
                <b style={role ? {color: ROLE_COLORS[role]} : undefined}>
                    <span className={am('nm')}>{name}</span>
                    {user.is_bot && <span className={am('bot-tag')}>{'BOT'}</span>}
                </b>
                <span><TalkNote userId={user.id}>{custom?.text || user.position || ''}</TalkNote></span>
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
    const groups = useMemberGroups(profiles, roles);
    const canAdd = useCanAddPeople(channel);
    const [popover, setPopover] = useState<{userId: string; anchor: HTMLElement} | null>(null);
    const [adding, setAdding] = useState(false);
    const page = useRef(0);
    const loading = useRef(false);
    const [userMenu, openUserMenu] = useUserMenu();
    const addRef = useRef<HTMLButtonElement>(null);

    useEffect(() => {
        if (channelId) {
            page.current = 0;
            dispatch(loadProfilesAndReloadChannelMembers(0, PER_PAGE, channelId));
            dispatch(getChannelStats(channelId));
        }
    }, [channelId, dispatch]);

    const direct = channel?.type === 'D' || channel?.type === 'G';

    const list = (
        <>
            <div className={am('mem-head')}>
                <span>{formatMessage({id: 'fusion.members.title', defaultMessage: 'Members — {count}'}, {count: Math.max(memberCount ?? 0, profiles.length)})}</span>
                {!direct && (canAdd.add || canAdd.invite) && (
                    <button
                        ref={addRef}
                        className={am('icon-btn', 'mem-add')}
                        title={formatMessage({id: 'fusion.members.add', defaultMessage: 'Add people or invite'})}
                        aria-label={formatMessage({id: 'fusion.members.addLabel', defaultMessage: 'Add people or create an invite'})}
                        aria-haspopup='menu'
                        aria-expanded={adding}
                        onClick={() => setAdding(!adding)}
                    >
                        <Icon
                            name='plus'
                            size='sm'
                        />
                    </button>
                )}
                {inPanel && (
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
            {groups.map((group) => (
                <React.Fragment key={group.key}>
                    <div className={am('role-head')}>
                        {group.color && (
                            <span
                                className={am('sw')}
                                style={{background: group.color}}
                            />
                        )}
                        {group.label}
                    </div>
                    {group.users.map((u) => (
                        <Member
                            key={u.id}
                            user={u}
                            off={group.off}
                            role={roles[u.id]}
                            onOpen={(anchor) => setPopover({userId: u.id, anchor})}
                            onMenu={(e) => openUserMenu(u.id, e)}
                        />
                    ))}
                </React.Fragment>
            ))}
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
