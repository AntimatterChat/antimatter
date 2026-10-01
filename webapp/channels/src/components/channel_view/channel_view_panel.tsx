// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import classNames from 'classnames';
import React from 'react';
import {useSelector} from 'react-redux';

import {getChannel} from 'mattermost-redux/selectors/entities/channels';

import PluggableErrorBoundary from 'plugins/pluggable/error_boundary';

import type {GlobalState} from 'types/store';
import type {ChannelViewPanelRegistration} from 'types/store/plugins';

import './channel_view_panel.scss';

type Props = {
    registration: ChannelViewPanelRegistration;
    channelId: string;
    messagesVisible: boolean;
    setMessagesVisible: (visible: boolean) => void;
};

/**
 * The panel a plugin shows at the top of the channel view (see registerChannelViewPanel). It fills
 * the channel view while the messages are hidden.
 */
export default function ChannelViewPanel({registration, channelId, messagesVisible, setMessagesVisible}: Props) {
    const channel = useSelector((state: GlobalState) => getChannel(state, channelId));
    if (!channel) {
        return null;
    }

    const Component = registration.component;
    return (
        <div
            className={classNames('ChannelViewPanel', {'ChannelViewPanel--fill': !messagesVisible})}
            data-testid='channel-view-panel'
        >
            <PluggableErrorBoundary
                key={`${registration.id}:${channel.id}`}
                pluginId={registration.pluginId}
            >
                <Component
                    channel={channel}
                    messagesVisible={messagesVisible}
                    setMessagesVisible={setMessagesVisible}
                />
            </PluggableErrorBoundary>
        </div>
    );
}
