// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useId} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {getChannelStats} from 'mattermost-redux/actions/channels';
import {getAllChannelStats, getDirectTeammate} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentRelativeTeamUrl} from 'mattermost-redux/selectors/entities/teams';

import Markdown from 'components/markdown';

import Avatar from 'fusion/components/avatar';
import {Popover} from 'fusion/components/layer';
import {MenuKeys} from 'fusion/components/menu';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import CopyIdItem from 'fusion/popovers/copy_id_item';
import ChannelIcon from 'fusion/sidebar/channel_icon';
import {am} from 'fusion/utils/class_names';
import Constants from 'utils/constants';
import {handleFormattedTextClick} from 'utils/utils';

import type {GlobalState} from 'types/store';

import {ChannelActionGroups, useChannelActions} from './channel_actions';

type Props = {
    channel: Channel;
    title: string;
    anchor: HTMLElement | null;
    onClose: () => void;
};

// ChannelNameMenu is the menu under a conversation's name in the chat header: what it is (its name, kind and
// purpose, else its topic), then its actions, as its sidebar menu has them (see useChannelActions): the frequent
// ones, its settings, the apps' actions, and leaving or archiving it at the bottom. Its members, pinned messages and
// files are the header's own buttons, and what the settings dialog changes is in that dialog.
export default function ChannelNameMenu({channel, title, anchor, onClose}: Props) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const dispatch = useDispatch();
    const headId = useId();
    const teamUrl = useSelector(getCurrentRelativeTeamUrl);
    const stats = useSelector((state: GlobalState) => getAllChannelStats(state)[channel.id]);
    const teammate = useSelector((state: GlobalState) => (channel.type === Constants.DM_CHANNEL ? getDirectTeammate(state, channel.id) : undefined));
    const creator = useUser(channel.creator_id);
    const creatorName = useDisplayName(creator);
    const {actions, replacement} = useChannelActions(channel, onClose);
    const direct = channel.type === Constants.DM_CHANNEL || channel.type === Constants.GM_CHANNEL;
    const archived = channel.delete_at !== 0;

    useEffect(() => {
        if (!direct) {
            dispatch(getChannelStats(channel.id, true));
        }
    }, [channel.id, direct, dispatch]);

    if (replacement) {
        return <>{replacement}</>;
    }

    let glyph: React.ReactNode;
    if (channel.type === Constants.DM_CHANNEL && teammate) {
        glyph = (
            <Avatar
                userId={teammate.id}
                size='sm'
                status={true}
            />
        );
    } else if (channel.type === Constants.GM_CHANNEL) {
        glyph = <span className={am('av', 'group', 'sm')}>{channel.display_name.split(',').length}</span>;
    } else {
        glyph = <ChannelIcon channel={channel}/>;
    }

    const meta = [
        !direct && creator ? formatMessage({id: 'fusion.info.created', defaultMessage: 'Created by {name} on {date}'}, {name: creatorName, date: intl.formatDate(channel.create_at, {dateStyle: 'long'})}) : '',
        !direct && stats ? formatMessage({id: 'fusion.browse.members', defaultMessage: '{count, plural, one {# member} other {# members}}'}, {count: stats.member_count}) : '',
    ].filter(Boolean).join(' · ');

    // The purpose says what the channel is for; without one, its topic (the header, in Markdown) does.
    let about: React.ReactNode = null;
    if (channel.purpose) {
        about = <p className={am('cm-about')}>{channel.purpose}</p>;
    } else if (channel.header) {
        about = (
            <div
                className={am('cm-about')}
                onClick={(e) => handleFormattedTextClick(e, teamUrl)}
            >
                <Markdown message={channel.header}/>
            </div>
        );
    }

    return (
        <Popover
            anchor={anchor}
            placement='below-start'
            className='plus-pop ch-menu ch-name-menu'
            role='menu'
            label={formatMessage({id: 'fusion.channelMenu.label', defaultMessage: 'Options for {name}'}, {name: title})}
            describedBy={headId}
            onClose={onClose}
        >
            <MenuKeys>
                <div
                    id={headId}
                    className={am('cm-head')}
                    role='none'
                >
                    <div className={am('cm-title')}>
                        {glyph}
                        <b>{title}</b>
                        {archived && <span className={am('cm-chip')}>{formatMessage({id: 'fusion.browse.isArchived', defaultMessage: 'Archived'})}</span>}
                    </div>
                    {about}
                    {meta && <p className={am('cm-meta')}>{meta}</p>}
                </div>
                <ChannelActionGroups
                    actions={actions}
                    onClose={onClose}
                />
                <CopyIdItem
                    id={channel.id}
                    onDone={onClose}
                />
            </MenuKeys>
        </Popover>
    );
}
