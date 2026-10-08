// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {favoriteChannel, getChannelStats, unfavoriteChannel} from 'mattermost-redux/actions/channels';
import {getAllChannelStats, getMyChannelMembership, isFavoriteChannel} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentRelativeTeamUrl, getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {isChannelMuted} from 'mattermost-redux/utils/channel_utils';

import ChannelInfoAttributes from 'components/channel_attributes/channel_info_attributes';
import ChannelSettingsModal from 'components/channel_settings_modal/channel_settings_modal';
import Markdown from 'components/markdown';

import Icon from 'fusion/components/icon';
import type {IconName} from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import type {Tab} from 'fusion/popovers/collections_popover';
import {useDialogs} from 'fusion/shell/dialogs_context';
import {useToast} from 'fusion/shell/toast_context';
import {muteFor, unmute} from 'fusion/sidebar/timed_mute';
import {am} from 'fusion/utils/class_names';
import {openDialog} from 'fusion/utils/modals';
import {channelPath} from 'fusion/utils/paths';
import Constants, {ModalIdentifiers} from 'utils/constants';
import {getSiteURL} from 'utils/url';
import {copyToClipboard, handleFormattedTextClick} from 'utils/utils';

import type {GlobalState} from 'types/store';

type Props = {
    channel: Channel;
    title: string;
    anchor: HTMLElement | null;
    onCollection: (tab: Tab) => void;
    onMembers: () => void;
    onClose: () => void;
};

// ChannelInfo is what the classic channel info panel shows, as a menu under the channel's name: its purpose and
// header, who made it and when, quick actions, its attributes, and its members, pinned messages and files.
export default function ChannelInfo({channel, title, anchor, onCollection, onMembers, onClose}: Props) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const dispatch = useDispatch();
    const dialogs = useDialogs();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const teamUrl = useSelector(getCurrentRelativeTeamUrl);
    const stats = useSelector((state: GlobalState) => getAllChannelStats(state)[channel.id]);
    const favorite = useSelector((state: GlobalState) => isFavoriteChannel(state, channel.id));
    const muted = useSelector((state: GlobalState) => isChannelMuted(getMyChannelMembership(state, channel.id)));
    const creator = useUser(channel.creator_id);
    const creatorName = useDisplayName(creator);
    const direct = channel.type === Constants.DM_CHANNEL || channel.type === Constants.GM_CHANNEL;
    const archived = channel.delete_at !== 0;

    useEffect(() => {
        dispatch(getChannelStats(channel.id, true));
    }, [channel.id, dispatch]);

    const action = (icon: IconName, label: string, onClick: () => void, on = false) => (
        <button
            type='button'
            className={am('info-act', {on})}
            onClick={onClick}
        >
            <Icon name={icon}/>
            <span>{label}</span>
        </button>
    );
    const link = (icon: IconName, label: string, count: number | undefined, onClick: () => void) => (
        <button
            type='button'
            role='menuitem'
            className={am('info-link')}
            onClick={onClick}
        >
            <Icon
                name={icon}
                size='sm'
            />
            <span className={am('grow')}>{label}</span>
            {count !== undefined && <span className={am('count')}>{count}</span>}
            <Icon
                name='chev'
                size='xs'
            />
        </button>
    );

    return (
        <Popover
            anchor={anchor}
            placement='below-start'
            className='info-pop'
            label={formatMessage({id: 'fusion.info.label', defaultMessage: 'About {name}'}, {name: title})}
            onClose={onClose}
        >
            <div className={am('info-head')}>
                <h3>{direct ? title : `#${title}`}</h3>
                {!direct && creator && (
                    <p className={am('info-sub')}>
                        {formatMessage({id: 'fusion.info.created', defaultMessage: 'Created by {name} on {date}'}, {name: creatorName, date: intl.formatDate(channel.create_at, {dateStyle: 'long'})})}
                    </p>
                )}
            </div>
            <div className={am('info-acts')}>
                {action('star', favorite ? formatMessage({id: 'fusion.info.favorited', defaultMessage: 'Favorite'}) : formatMessage({id: 'fusion.info.favorite', defaultMessage: 'Favorite'}), () => dispatch(favorite ? unfavoriteChannel(channel.id) : favoriteChannel(channel.id)), favorite)}
                {action(muted ? 'bell-off' : 'bell', muted ? formatMessage({id: 'fusion.info.muted', defaultMessage: 'Muted'}) : formatMessage({id: 'fusion.info.mute', defaultMessage: 'Mute'}), () => dispatch(muted ? unmute(channel.id) : muteFor(channel.id, 0)), muted)}
                {!direct && !archived && action('user-plus', formatMessage({id: 'fusion.info.add', defaultMessage: 'Add people'}), () => {
                    onClose();
                    dialogs.addPeople(channel);
                })}
                {action('link', formatMessage({id: 'fusion.info.copyLink', defaultMessage: 'Copy link'}), () => {
                    if (team) {
                        copyToClipboard(getSiteURL() + channelPath(team.name, channel));
                        toast(formatMessage({id: 'fusion.toast.linkCopied', defaultMessage: 'Link copied'}));
                    }
                })}
                {!direct && !archived && action('cog', formatMessage({id: 'fusion.info.settings', defaultMessage: 'Settings'}), () => {
                    onClose();
                    dispatch(openDialog(ModalIdentifiers.CHANNEL_SETTINGS, ChannelSettingsModal, {channelId: channel.id, isOpen: true}));
                })}
            </div>
            {(channel.purpose || channel.header) && (
                <div
                    className={am('info-about')}
                    onClick={(e) => handleFormattedTextClick(e, teamUrl)}
                >
                    {channel.purpose && (
                        <section>
                            <h5>{formatMessage({id: 'fusion.header.purpose', defaultMessage: 'Purpose'})}</h5>
                            <p>{channel.purpose}</p>
                        </section>
                    )}
                    {channel.header && (
                        <section>
                            <h5>{formatMessage({id: 'fusion.header.header', defaultMessage: 'Header'})}</h5>
                            <Markdown message={channel.header}/>
                        </section>
                    )}
                </div>
            )}
            {!direct && (
                <div className={am('info-attrs')}>
                    <ChannelInfoAttributes channelId={channel.id}/>
                </div>
            )}
            <div className={am('info-links')}>
                {!direct && link('users', formatMessage({id: 'fusion.info.members', defaultMessage: 'Members'}), stats?.member_count, () => {
                    onClose();
                    onMembers();
                })}
                {link('pin', formatMessage({id: 'fusion.info.pinned', defaultMessage: 'Pinned messages'}), stats?.pinnedpost_count, () => onCollection('pinned'))}
                {link('file', formatMessage({id: 'fusion.info.files', defaultMessage: 'Files'}), stats?.files_count, () => onCollection('files'))}
            </div>
        </Popover>
    );
}
