// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {UserProfile} from '@mattermost/types/users';

import {getCurrentChannelId, getCurrentChannel} from 'mattermost-redux/selectors/entities/channels';
import {getStatusForUserId, makeGetProfilesInChannel} from 'mattermost-redux/selectors/entities/users';

import {loadProfilesAndReloadChannelMembers} from 'actions/user_actions';
import {closeRightHandSide} from 'actions/views/rhs';
import {makeGetCustomStatus} from 'selectors/views/custom_status';

import ChannelInviteModal from 'components/channel_invite_modal';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {useDisplayName} from 'fusion/hooks/users';
import UserPopover from 'fusion/popovers/user_popover';
import {am} from 'fusion/utils/class_names';
import {openDialog} from 'fusion/utils/modals';
import {ModalIdentifiers} from 'utils/constants';

import type {GlobalState} from 'types/store';

function Member({user, off, onOpen}: {user: UserProfile; off: boolean; onOpen: (el: HTMLElement) => void}) {
    const name = useDisplayName(user);
    const getCustomStatus = useMemo(() => makeGetCustomStatus(), []);
    const custom = useSelector((state: GlobalState) => getCustomStatus(state, user.id));
    return (
        <button
            className={am('member', {off})}
            onClick={(e) => onOpen(e.currentTarget)}
        >
            <Avatar
                userId={user.id}
                status={true}
            />
            <span className={am('who')}>
                <b>
                    <span className={am('nm')}>{name}</span>
                    {user.is_bot && <span className={am('bot-tag')}>{'BOT'}</span>}
                </b>
                <span>{custom?.text || user.position || ''}</span>
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
    const statuses = useSelector((state: GlobalState) => Object.fromEntries(profiles.map((u) => [u.id, getStatusForUserId(state, u.id)])), shallowEqual);
    const [popover, setPopover] = useState<{userId: string; anchor: HTMLElement} | null>(null);
    const addRef = useRef<HTMLButtonElement>(null);

    useEffect(() => {
        if (channelId) {
            dispatch(loadProfilesAndReloadChannelMembers(0, 100, channelId));
        }
    }, [channelId, dispatch]);

    const online = profiles.filter((u) => statuses[u.id] && statuses[u.id] !== 'offline');
    const offline = profiles.filter((u) => !statuses[u.id] || statuses[u.id] === 'offline');
    const direct = channel?.type === 'D' || channel?.type === 'G';

    const list = (
        <>
            <div className={am('mem-head')}>
                <span>{formatMessage({id: 'fusion.members.title', defaultMessage: 'Members — {count}'}, {count: profiles.length})}</span>
                {!direct && (
                    <button
                        ref={addRef}
                        className={am('icon-btn', 'mem-add')}
                        title={formatMessage({id: 'fusion.members.add', defaultMessage: 'Add people'})}
                        aria-label={formatMessage({id: 'fusion.members.add', defaultMessage: 'Add people'})}
                        onClick={() => channel && dispatch(openDialog(ModalIdentifiers.CHANNEL_INVITE, ChannelInviteModal, {channel}))}
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
            {online.length > 0 && (
                <>
                    <div className={am('role-head')}>{formatMessage({id: 'fusion.members.online', defaultMessage: 'Online — {count}'}, {count: online.length})}</div>
                    {online.map((u) => (
                        <Member
                            key={u.id}
                            user={u}
                            off={false}
                            onOpen={(anchor) => setPopover({userId: u.id, anchor})}
                        />
                    ))}
                </>
            )}
            {offline.length > 0 && (
                <>
                    <div className={am('role-head')}>{formatMessage({id: 'fusion.members.offline', defaultMessage: 'Offline — {count}'}, {count: offline.length})}</div>
                    {offline.map((u) => (
                        <Member
                            key={u.id}
                            user={u}
                            off={true}
                            onOpen={(anchor) => setPopover({userId: u.id, anchor})}
                        />
                    ))}
                </>
            )}
            {popover && (
                <UserPopover
                    userId={popover.userId}
                    anchor={popover.anchor}
                    onClose={() => setPopover(null)}
                />
            )}
        </>
    );

    if (inPanel) {
        return <div className={am('rhs-body', 'members-panel')}>{list}</div>;
    }
    return (
        <aside
            className={am('members')}
            aria-label={formatMessage({id: 'fusion.members.label', defaultMessage: 'Members'})}
        >
            {list}
        </aside>
    );
}
