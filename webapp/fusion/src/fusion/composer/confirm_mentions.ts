// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {getChannelStats, getChannelTimezones} from 'mattermost-redux/actions/channels';
import {Permissions} from 'mattermost-redux/constants';
import {getAllChannelStats, getChannel} from 'mattermost-redux/selectors/entities/channels';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {haveIChannelPermission} from 'mattermost-redux/selectors/entities/roles';

import NotifyConfirmModal from 'components/notify_confirm_modal';

import {openDialog} from 'fusion/utils/modals';
import Constants, {ModalIdentifiers} from 'utils/constants';
import {specialMentionsInText} from 'utils/post_utils';

import type {ActionFuncAsync} from 'types/store';

// confirmChannelWideMentions asks before a message notifies a whole channel of more than a few people with @all,
// @channel or @here, saying how many people and how many timezones it reaches, as the classic message box does when
// the server asks for it (EnableConfirmNotificationsToChannel). Resolves to whether to send the message.
export function confirmChannelWideMentions(channelId: string, message: string): ActionFuncAsync<boolean> {
    return async (dispatch, getState) => {
        const state = getState();
        const channel = getChannel(state, channelId);
        if (!channel || getConfig(state).EnableConfirmNotificationsToChannel !== 'true' ||
            !haveIChannelPermission(state, channel.team_id, channel.id, Permissions.USE_CHANNEL_MENTIONS)) {
            return {data: true};
        }

        const special = specialMentionsInText(message);
        const mentions = Object.keys(special).filter((name) => special[name]).map((name) => '@' + name);
        if (!mentions.length) {
            return {data: true};
        }

        let members = getAllChannelStats(state)[channelId]?.member_count;
        if (members === undefined) {
            await dispatch(getChannelStats(channelId));
            members = getAllChannelStats(getState())[channelId]?.member_count ?? 1;
        }
        if (members <= Constants.NOTIFY_ALL_MEMBERS) {
            return {data: true};
        }

        const {data: timezones} = await dispatch(getChannelTimezones(channelId));
        return new Promise((resolve) => {
            let confirmed = false;
            dispatch(openDialog(ModalIdentifiers.NOTIFY_CONFIRM_MODAL, NotifyConfirmModal, {
                mentions,
                memberNotifyCount: members! - 1,
                channelTimezoneCount: timezones ? timezones.length : 0,
                onConfirm: () => {
                    confirmed = true;
                    resolve({data: true});
                },
                onExited: () => {
                    if (!confirmed) {
                        resolve({data: false});
                    }
                },
            }));
        });
    };
}
