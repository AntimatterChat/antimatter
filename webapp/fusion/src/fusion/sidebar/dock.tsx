// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo, useRef} from 'react';
import type {RefObject} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useSelector} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {makeGetChannel} from 'mattermost-redux/selectors/entities/channels';

import {useCallsOnTop} from 'fusion/calls/calls_on_top';
import {useLayout} from 'fusion/shell/layout_context';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import DirectRow from './direct_row';
import {makeGetDockChannelIds} from './dock_channels';
import {useDockFit} from './dock_fit';

type Props = {

    /** The sidebar and its channel list, to measure the room the dock has. */
    sidebarRef: RefObject<HTMLElement | null>;
    scrollRef: RefObject<HTMLElement | null>;
};

// Dock shows the direct messages category as a dock of open conversations at the bottom of the sidebar. It never
// scrolls: it lists as many conversations as fit (useDockFit), up to the Open conversations setting, the one you're in
// first, then the latest active. The unread ones left out wait on the team rail, under the chat bubble that opens all
// the conversations.
export default function Dock({sidebarRef, scrollRef}: Props) {
    const {formatMessage} = useIntl();
    const dockRef = useRef<HTMLDivElement>(null);
    useDockFit({sidebar: sidebarRef, scroll: scrollRef, dock: dockRef});
    const {dockRows} = useLayout();

    // The open conversations, unread or not: the Unreads section above the categories leaves them here, and the team
    // rail leaves them out of its unread bubbles.
    const getDockChannelIds = useMemo(() => makeGetDockChannelIds(), []);
    const channelIds = useSelector((state: GlobalState) => getDockChannelIds(state, dockRows));
    const getChannel = useMemo(() => makeGetChannel(), []);
    const open = useSelector((state: GlobalState) => channelIds.map((id) => getChannel(state, id)).filter(Boolean) as Channel[], shallowEqual);

    // A conversation with a call in progress stays on top while it lasts.
    const channels = useCallsOnTop(open);

    return (
        <div
            ref={dockRef}
            className={am('dock')}
            aria-label={formatMessage({id: 'fusion.dock.label', defaultMessage: 'Open conversations'})}
        >
            <div className={am('dock-head')}>
                <span className={am('grow')}>{formatMessage({id: 'fusion.dock.title', defaultMessage: 'Open conversations'})}</span>
            </div>
            {channels.length > 0 && (
                <div
                    className={am('dock-list')}
                    style={{overflowY: 'hidden'}}
                >
                    {channels.map((channel) => (
                        <DirectRow
                            key={channel.id}
                            channel={channel}
                            dock={true}
                        />
                    ))}
                </div>
            )}
        </div>
    );
}
