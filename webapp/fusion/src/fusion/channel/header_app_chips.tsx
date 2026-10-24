// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Channel} from '@mattermost/types/channels';

import {am} from 'fusion/utils/class_names';
import Pluggable from 'plugins/pluggable';

// HeaderAppChips is where apps show a short status chip about the conversation beside its name in the chat header
// (registerChannelHeaderChip), as Fusion's own "Has guests" and "Restricted" chips: for example the channel's next
// scheduled call. Each app's chip gets the channel and decides whether to show anything; with none, the place
// collapses (:empty). Apps can give their chip Fusion's look with the am-head-chip class (and am-live while
// something is on).
export default function HeaderAppChips({channel}: {channel: Channel}) {
    return (
        <span className={am('head-chips')}>
            <Pluggable
                pluggableName='ChannelHeaderChip'
                channel={channel}
            />
        </span>
    );
}
