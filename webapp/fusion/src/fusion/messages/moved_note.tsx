// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage, useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {getChannel} from 'mattermost-redux/selectors/entities/channels';

import {renderWranglerSystemMessage} from 'components/post_markdown/system_message_helpers';

import {permalinkPath} from 'fusion/utils/paths';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

// The props the server puts on a moved thread's pointer (MovedThread…, where it went) and note (MovedFrom…, where it
// came from). A private channel's name and display name aren't there.
type Moved = {channelId: string; name?: string; displayName?: string; teamName?: string; postId?: string; link?: string};

function str(value: unknown): string | undefined {
    return typeof value === 'string' && value ? value : undefined;
}

export function movedTo(post: Post): Moved | undefined {
    const props = post.props || {};
    const channelId = str(props.MovedThreadChannelId);
    return channelId ? {
        channelId,
        name: str(props.MovedThreadChannelName),
        displayName: str(props.MovedThreadChannelDisplayName),
        teamName: str(props.MovedThreadTeamName),
        postId: str(props.MovedThreadPostId),
        link: str(props.MovedThreadPermalink),
    } : undefined;
}

export function movedFrom(post: Post): Moved | undefined {
    const props = post.props || {};
    const channelId = str(props.MovedFromChannelId);
    return channelId ? {
        channelId,
        name: str(props.MovedFromChannelName),
        displayName: str(props.MovedFromChannelDisplayName),
        teamName: str(props.MovedFromTeamName),
        postId: str(props.MovedFromPostId),
        link: str(props.MovedFromPermalink),
    } : undefined;
}

// MovedChannelLink is the channel a thread went to or came from, as a channel link (#name) that opens the moved
// thread or the place it left. A channel the reader doesn't know (a private one they aren't in) is "another
// channel"; the link stays, and refuses them.
function MovedChannelLink({moved}: {moved: Moved}) {
    const {formatMessage} = useIntl();
    const known = useSelector((state: GlobalState) => getChannel(state, moved.channelId));
    const name = known?.display_name || moved.displayName;
    const label = name ? '#' + name : formatMessage({id: 'fusion.moved.anotherChannel', defaultMessage: 'another channel'});
    const path = moved.teamName && moved.postId ? permalinkPath(moved.teamName, moved.postId) : undefined;
    return (
        <a
            className='mention-link'
            href={path || moved.link || '#'}
            data-channel-mention={known?.name || moved.name}
            onClick={(e) => {
                if (path) {
                    e.preventDefault();
                    getHistory().push(path);
                }
            }}
        >
            {label}
        </a>
    );
}

// MovedNote is the text of a moved thread's pointer ("A thread with 13 messages has been moved to #channel") or
// note ("This thread was moved from #channel"), with the channel as a link, never a bare address. Pointers and notes
// left before Antimatter named the channel keep their old text.
export default function MovedNote({post}: {post: Post}) {
    const to = movedTo(post);
    if (to) {
        const count = Number(post.props?.NumMessages) || 1;
        return (
            <FormattedMessage
                id='fusion.moved.to'
                defaultMessage='{count, plural, one {A message has been moved to {channel}} other {A thread with # messages has been moved to {channel}}}'
                values={{count, channel: <MovedChannelLink moved={to}/>}}
            />
        );
    }
    const from = movedFrom(post);
    if (from) {
        return (
            <FormattedMessage
                id='fusion.moved.from'
                defaultMessage='This thread was moved from {channel}'
                values={{channel: <MovedChannelLink moved={from}/>}}
            />
        );
    }
    return <>{renderWranglerSystemMessage(post)}</>;
}
