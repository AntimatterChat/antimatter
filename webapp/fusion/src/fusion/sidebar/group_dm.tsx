// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import ConvertGmToChannelModal from 'components/convert_gm_to_channel_modal';

import {Dialog, Popover} from 'fusion/components/layer';
import {MenuItem} from 'fusion/components/menu';
import {am} from 'fusion/utils/class_names';
import {openDialog} from 'fusion/utils/modals';
import {ModalIdentifiers} from 'utils/constants';

import type {ActionFuncAsync, GlobalState} from 'types/store';

// Mattermost names group messages after their members and doesn't let them be renamed, so the name one gives a group
// message is one's own: a preference, which only they see.
const CATEGORY_GROUP_NAMES = 'fusion_group_names';

export function getGroupName(state: GlobalState, channelId: string): string {
    return getPreference(state, CATEGORY_GROUP_NAMES, channelId, '');
}

export function setGroupName(channelId: string, name: string): ActionFuncAsync {
    return (dispatch, getState) => {
        const userId = getCurrentUserId(getState());
        return dispatch(savePreferences(userId, [{user_id: userId, category: CATEGORY_GROUP_NAMES, name: channelId, value: name.trim()}]));
    };
}

// useConversationName is the name to show for a conversation: the one given to a group message, else its own.
export function useConversationName(channel: Pick<Channel, 'id' | 'type' | 'display_name'>): string {
    const custom = useSelector((state: GlobalState) => (channel.type === 'G' ? getGroupName(state, channel.id) : ''));
    return custom || channel.display_name;
}

// RenameGroupDialog names a group message, for oneself; an empty name goes back to its members' names.
export function RenameGroupDialog({channel, onClose}: {channel: Channel; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const current = useSelector((state: GlobalState) => getGroupName(state, channel.id));
    const [name, setName] = useState(current);
    const title = formatMessage({id: 'fusion.group.renameTitle', defaultMessage: 'Rename group'});

    return (
        <Dialog
            label={title}
            onClose={onClose}
            onSubmit={() => {
                dispatch(setGroupName(channel.id, name));
                onClose();
            }}
        >
            <div className={am('pane')}>
                <h2>{title}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.group.renameLead', defaultMessage: 'Only you see this name. Leave it empty to show the members\' names again.'})}</p>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.group.name', defaultMessage: 'Name'})}</span>
                    <input
                        type='text'
                        value={name}
                        placeholder={channel.display_name}
                        maxLength={64}
                        autoFocus={true}
                        onChange={(e) => setName(e.target.value)}
                    />
                </label>
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.group.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button className={am('btn', 'primary')}>
                        {formatMessage({id: 'fusion.group.save', defaultMessage: 'Save'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}

// GroupMenu is the right-click menu of a group message: rename it, or turn it into a private channel of a team, as
// Mattermost's "Convert to private channel" does.
export function GroupMenu({channel, point, onClose}: {channel: Channel; point: {x: number; y: number}; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const [renaming, setRenaming] = useState(false);

    if (renaming) {
        return (
            <RenameGroupDialog
                channel={channel}
                onClose={onClose}
            />
        );
    }
    return (
        <Popover
            point={point}
            placement='point'
            className='plus-pop ch-menu'
            role='menu'
            label={formatMessage({id: 'fusion.group.menu', defaultMessage: 'Group options'})}
            onClose={onClose}
        >
            <MenuItem
                icon='pen'
                label={formatMessage({id: 'fusion.group.rename', defaultMessage: 'Rename group'})}
                onClick={() => setRenaming(true)}
            />
            <MenuItem
                icon='hash'
                label={formatMessage({id: 'fusion.group.convert', defaultMessage: 'Convert to private channel'})}
                onClick={() => {
                    dispatch(openDialog(ModalIdentifiers.CONVERT_GM_TO_CHANNEL, ConvertGmToChannelModal, {channel}));
                    onClose();
                }}
            />
        </Popover>
    );
}
