// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Channel} from '@mattermost/types/channels';

// Every kind of channel, direct and group messages included, resolves from /<team>/channels/<name>.
export function channelPath(teamName: string, channel: Pick<Channel, 'name'>): string {
    return `/${teamName}/channels/${channel.name}`;
}

export function permalinkPath(teamName: string, postId: string): string {
    return `/${teamName}/pl/${postId}`;
}

// The address's state once a permalink's message is no longer in it (a few seconds after it opened): the message
// the conversation's list stays around.
export type PermalinkAnchorState = {permalinkAnchor?: string};

// A forum topic's own address (Antimatter channel types, design §4.4): the topics view (which replaces the message
// list in a forum channel) is left for the topic page, first post and replies, at this URL.
export function topicPath(teamName: string, channel: Pick<Channel, 'name'>, rootId: string): string {
    return `${channelPath(teamName, channel)}/topics/${rootId}`;
}
