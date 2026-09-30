// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Channel} from '@mattermost/types/channels';

import {am} from 'fusion/utils/class_names';
import PluggableErrorBoundary from 'plugins/pluggable/error_boundary';

import type {ChannelViewPanelRegistration} from 'types/store/plugins';

type Props = {
    registration: ChannelViewPanelRegistration;
    channel: Channel;
    messagesVisible: boolean;
    setMessagesVisible: (visible: boolean) => void;
};

// ChannelViewPanel is the panel a plugin shows above the messages (see registerChannelViewPanel),
// e.g. the call of a voice channel. It fills the view while the messages are hidden.
export default function ChannelViewPanel({registration, channel, messagesVisible, setMessagesVisible}: Props) {
    const Component = registration.component;
    return (
        <div
            className={am('view-panel', {fill: !messagesVisible})}
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
