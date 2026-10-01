// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback} from 'react';
import {useSelector} from 'react-redux';

import {ReplyOutlineIcon} from '@mattermost/compass-icons/components';
import type {Post} from '@mattermost/types/posts';

import {getCurrentRelativeTeamUrl} from 'mattermost-redux/selectors/entities/teams';

import {getHistory} from 'utils/browser_history';
import {getReplyToId, isInlineRepliesEnabled} from 'utils/inline_replies';

import QuotedMessage from './quoted_message';

import './inline_reply.scss';

type Props = {
    post: Post;
};

// InlineReplyQuote is shown above an inline reply: "Replying to Name: snippet", where the snippet links to the quoted
// message (its permalink scrolls to it and highlights it).
export default function InlineReplyQuote({post}: Props) {
    const enabled = useSelector(isInlineRepliesEnabled);
    const teamUrl = useSelector(getCurrentRelativeTeamUrl);
    const quotedId = getReplyToId(post);

    const jump = useCallback((e: React.MouseEvent) => {
        e.preventDefault();
        getHistory().push(`${teamUrl}/pl/${quotedId}`);
    }, [teamUrl, quotedId]);

    if (!enabled || !quotedId) {
        return null;
    }

    return (
        <div
            className='InlineReplyQuote'
            data-testid='post-inline-reply'
        >
            <ReplyOutlineIcon
                size={14}
                className='InlineReplyQuote__icon'
            />
            <QuotedMessage
                postId={quotedId}
                described={post.metadata?.reply_to}
                onClick={jump}
            />
        </div>
    );
}
