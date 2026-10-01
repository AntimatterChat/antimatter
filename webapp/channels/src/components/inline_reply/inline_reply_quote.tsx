// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import classNames from 'classnames';
import React, {useCallback} from 'react';
import {FormattedMessage, useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {PaperclipIcon, ReplyOutlineIcon} from '@mattermost/compass-icons/components';
import type {Post} from '@mattermost/types/posts';

import {getTheme} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentRelativeTeamUrl} from 'mattermost-redux/selectors/entities/teams';
import {getUser} from 'mattermost-redux/selectors/entities/users';

import {generateColor} from 'components/user_profile/utils';
import Avatar from 'components/widgets/users/avatar';

import {getHistory} from 'utils/browser_history';
import {getReplyToId, isInlineRepliesEnabled} from 'utils/inline_replies';
import {imageURLForUser} from 'utils/utils';

import type {GlobalState} from 'types/store';

import {quotedText, useQuoted} from './quoted_message';

import './inline_reply.scss';

type Props = {
    post: Post;

    // The compact message display has no pictures, so no spine: the line starts with a reply arrow instead.
    compact?: boolean;

    // Whether to colour the quoted author's name, as the compact display's names can be.
    colorize?: boolean;
};

// InlineReplyQuote is the line above an inline reply, as in Discord: the quoted author's small picture, their name in
// bold and the start of their message, which InlineReplySpine joins to the reply's picture. Clicking it opens the
// quoted message's permalink, which scrolls to it and highlights it.
export default function InlineReplyQuote({post, compact = false, colorize = false}: Props) {
    const {formatMessage} = useIntl();
    const enabled = useSelector(isInlineRepliesEnabled);
    const teamUrl = useSelector(getCurrentRelativeTeamUrl);
    const quotedId = getReplyToId(post);
    const {quoted, name} = useQuoted(quotedId, post.metadata?.reply_to);
    const author = useSelector((state: GlobalState) => (quoted.userId ? getUser(state, quoted.userId) : undefined));
    const theme = useSelector(getTheme);

    const jump = useCallback((e: React.MouseEvent) => {
        e.preventDefault();
        getHistory().push(`${teamUrl}/pl/${quotedId}`);
    }, [teamUrl, quotedId]);

    if (!enabled || !quotedId) {
        return null;
    }

    let snippet: React.ReactNode = quotedText(quoted);
    let filesIcon;
    if (quoted.deleted) {
        snippet = (
            <FormattedMessage
                id='inline_reply.deleted'
                defaultMessage='Original message was deleted'
            />
        );
    } else if (!snippet && quoted.fileCount) {
        filesIcon = (
            <PaperclipIcon
                size={14}
                aria-hidden='true'
            />
        );
        snippet = (
            <FormattedMessage
                id='inline_reply.seeFiles'
                defaultMessage='Click to see attachment'
            />
        );
    } else if (!snippet) {
        snippet = (
            <FormattedMessage
                id='inline_reply.seeMessage'
                defaultMessage='Click to see message'
            />
        );
    }

    let picture;
    if (author && !quoted.deleted) {
        picture = (
            <Avatar
                size='xxs'
                url={imageURLForUser(author.id, author.last_picture_update)}
                alt=''
                className='InlineReplyQuote__avatar'
            />
        );
    } else {
        picture = (
            <span className='InlineReplyQuote__avatar InlineReplyQuote__avatar--none'>
                <ReplyOutlineIcon size={10}/>
            </span>
        );
    }

    const content = (
        <>
            <span className='sr-only'>
                {name ? formatMessage({id: 'inline_reply.quote.replyingTo', defaultMessage: 'Replying to {name}'}, {name}) : formatMessage({id: 'inline_reply.quote.replying', defaultMessage: 'Replying to a message'})}
            </span>
            {compact && (
                <ReplyOutlineIcon
                    size={14}
                    className='InlineReplyQuote__icon'
                    aria-hidden='true'
                />
            )}
            <span
                className='InlineReplyQuote__who'
                aria-hidden='true'
            >
                {picture}
                {name && (
                    <span
                        className='InlineReplyQuote__name'
                        style={colorize && author ? {color: generateColor(author.username, theme.centerChannelBg)} : undefined}
                    >
                        {name}
                    </span>
                )}
            </span>
            <span className={classNames('InlineReplyQuote__snippet', {'InlineReplyQuote__snippet--deleted': quoted.deleted, 'InlineReplyQuote__snippet--files': filesIcon})}>
                {filesIcon}
                {snippet}
            </span>
        </>
    );

    return (
        <div
            className={classNames('InlineReplyQuote', 'InlineReplyQuote--post', {'InlineReplyQuote--compact': compact})}
            data-testid='post-inline-reply'
        >
            {quoted.deleted ? (
                <span className='InlineReplyQuote__jump'>{content}</span>
            ) : (
                <button
                    type='button'
                    className='InlineReplyQuote__jump'
                    title={formatMessage({id: 'inline_reply.jump', defaultMessage: 'Jump to the original message'})}
                    onClick={jump}
                >
                    {content}
                </button>
            )}
        </div>
    );
}

// InlineReplySpine sits above an inline reply's picture, as tall as the InlineReplyQuote line beside it, and draws
// Discord's curved line from the picture up and right into that line.
export function InlineReplySpine() {
    return (
        <div
            className='InlineReplyQuote__spine'
            aria-hidden='true'
        />
    );
}
