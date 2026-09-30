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
