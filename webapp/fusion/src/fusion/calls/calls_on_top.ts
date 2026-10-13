// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useMemo} from 'react';
import {useSelector, useStore} from 'react-redux';

import type {Channel} from '@mattermost/types/channels';

import {getChannel} from 'mattermost-redux/selectors/entities/channels';

import type {GlobalState} from 'types/store';

import {getCallChannelIds, useCallsAPIs} from './calls_api';

// callsOnTop puts the direct and group messages with a call in progress first, in the order of the list, then the
// others as they were. Conversations with a call that the list leaves out (closed, or hidden) come back on top while
// the call lasts.
export function callsOnTop(channels: Channel[], callChannels: Channel[]): Channel[] {
    if (!callChannels.length) {
        return channels;
    }
    const withCall = new Set(callChannels.map((c) => c.id));
    const listed = new Set(channels.map((c) => c.id));
    return [
        ...channels.filter((c) => withCall.has(c.id)),
        ...callChannels.filter((c) => !listed.has(c.id)),
        ...channels.filter((c) => !withCall.has(c.id)),
    ];
}

// The direct and group messages with a call in progress, as a string to compare by value.
function directCallsKey(state: GlobalState): string {
    return getCallChannelIds(state).filter((id) => {
        const channel = getChannel(state, id);
        return (channel?.type === 'D' || channel?.type === 'G') && channel.delete_at === 0;
    }).join(',');
}

// useCallsOnTop is callsOnTop for a list of conversations: the conversations with a call stay on top while it lasts.
export function useCallsOnTop(channels: Channel[]): Channel[] {
    useCallsAPIs();
    const store = useStore<GlobalState>();
    const key = useSelector(directCallsKey);
    return useMemo(() => {
        if (!key) {
            return channels;
        }
        const state = store.getState();
        const callChannels = key.split(',').map((id) => getChannel(state, id)).filter(Boolean) as Channel[];
        return callsOnTop(channels, callChannels);
    }, [channels, key, store]);
}
