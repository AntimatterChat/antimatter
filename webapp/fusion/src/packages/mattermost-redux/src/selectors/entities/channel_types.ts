// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ChannelTypeEntry} from '@mattermost/types/channels';
import type {GlobalState} from '@mattermost/types/store';

// Antimatter channel types: a plain text channel, the type (and its default preset) every channel had before channel
// types, and still has when nothing says otherwise (see server/public/model/channel_type.go DefaultChannelTypeInfo).
export const DEFAULT_CHANNEL_TYPE: ChannelTypeEntry = {type: 'text', mode: 'chat', root_posts: 'members', preset: 'text_chat'};

// getChannelTypeInfo gives a channel's type and settings, from the Fusion channelTypes store (reducers/entities/
// channels.ts), falling back to the plain text default for a channel whose payload never carried props.am_type (not
// loaded yet, or a direct/group message).
export function getChannelTypeInfo(state: GlobalState, channelId: string): ChannelTypeEntry {
    return state.entities.channels.channelTypes?.[channelId] || DEFAULT_CHANNEL_TYPE;
}
