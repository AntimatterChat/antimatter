// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage} from 'react-intl';
import {shallowEqual, useSelector} from 'react-redux';

import type {PostReplyTo} from '@mattermost/types/posts';

import {getConfig} from 'mattermost-redux/selectors/entities/general';

import {useUser} from 'components/common/hooks/useUser';
import UserProfile from 'components/user_profile';

import {getQuotedMessage} from 'utils/inline_replies';
import type {QuotedMessage as Quoted} from 'utils/inline_replies';
import {stripMarkdown} from 'utils/markdown';
import * as Utils from 'utils/utils';

import type {GlobalState} from 'types/store';

const SNIPPET_LENGTH = 160;

// useQuoted is what to show of the message postId, which an inline reply quotes: the message, and the name of its
// author ('' while the author isn't loaded).
export function useQuoted(postId: string, described?: PostReplyTo): {quoted: Quoted; name: string} {
    const quoted = useSelector((state: GlobalState) => getQuotedMessage(state, postId, described), shallowEqual);
    const overrideAllowed = useSelector((state: GlobalState) => getConfig(state).EnablePostUsernameOverride === 'true');
    const author = useUser(quoted.userId || '');
    const displayName = useSelector((state: GlobalState) => (author ? Utils.getDisplayNameByUser(state, author) : ''));
    return {quoted, name: (overrideAllowed && quoted.overrideUsername) || displayName};
}

// quotedText is the start of a quoted message's text, without its markdown, or '' when it has none.
export function quotedText(quoted: Quoted): string {
    return quoted.message.trim() ? stripMarkdown(Utils.replaceHtmlEntities(quoted.message)).slice(0, SNIPPET_LENGTH) : '';
}

type Props = {
    postId: string;

    // The server's description of the quoted message, for when it isn't loaded.
    described?: PostReplyTo;

    onClick?: React.MouseEventHandler<HTMLAnchorElement>;
};

// QuotedMessage says "Replying to Name: snippet" for the message an inline reply quotes.
export default function QuotedMessage({postId, described, onClick}: Props) {
    const quoted = useSelector((state: GlobalState) => getQuotedMessage(state, postId, described), shallowEqual);
    const overrideAllowed = useSelector((state: GlobalState) => getConfig(state).EnablePostUsernameOverride === 'true');
    const author = useUser(quoted.userId || '');

    let snippet: React.ReactNode;
    if (quoted.deleted) {
        snippet = (
            <FormattedMessage
                id='inline_reply.deleted'
                defaultMessage='Original message was deleted'
            />
        );
    } else if (quoted.message.trim()) {
        snippet = stripMarkdown(Utils.replaceHtmlEntities(quoted.message)).slice(0, SNIPPET_LENGTH);
    } else if (quoted.fileCount) {
        snippet = (
            <FormattedMessage
                id='inline_reply.files'
                defaultMessage='{count, plural, one {# file} other {# files}}'
                values={{count: quoted.fileCount}}
            />
        );
    } else {
        snippet = (
            <FormattedMessage
                id='inline_reply.message'
                defaultMessage='a message'
            />
        );
    }

    let name: React.ReactNode = null;
    if (author || (overrideAllowed && quoted.overrideUsername)) {
        name = (
            <UserProfile
                userId={author?.id ?? ''}
                overwriteName={overrideAllowed ? quoted.overrideUsername : undefined}
            />
        );
    }

    return (
        <span className='InlineReplyQuote__text'>
            {name ? (
                <FormattedMessage
                    id='inline_reply.replyingTo'
                    defaultMessage='Replying to {name}: '
                    values={{name: <span className='InlineReplyQuote__name'>{name}</span>}}
                />
            ) : (
                <FormattedMessage
                    id='inline_reply.replying'
                    defaultMessage='Replying to: '
                />
            )}
            {quoted.deleted || !onClick ? (
                <span className={quoted.deleted ? 'InlineReplyQuote__snippet InlineReplyQuote__snippet--deleted' : 'InlineReplyQuote__snippet'}>{snippet}</span>
            ) : (
                <a
                    className='InlineReplyQuote__snippet theme'
                    onClick={onClick}
                >
                    {snippet}
                </a>
            )}
        </span>
    );
}
