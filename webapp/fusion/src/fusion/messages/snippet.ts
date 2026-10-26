// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useCallback} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {Posts} from 'mattermost-redux/constants';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {getBlockLists, isUserBlockingEnabled} from 'fusion/privacy/blocking';
import type {BlockLevel} from 'fusion/privacy/blocking';

import type {GlobalState} from 'types/store';

import {isSpoiler} from './spoiler';

// A snippet shows a message's text without drawing the message (the Unreads inbox, search results, quick search, the
// saved and pinned lists, link previews, the home list's last message). It keeps what the message itself keeps: a
// spoiler's text stays hidden, and so does the text of someone you ignore or block (the message folds).
export type SnippetCover = '' | 'spoiler' | Exclude<BlockLevel, ''>;

type CoverPost = Pick<Post, 'user_id'> & Partial<Pick<Post, 'props' | 'state'>>;

/** coverOf says what hides a message's text in a snippet, if anything: whom it's from first, then a spoiler. */
export function coverOf(post: CoverPost | undefined, me: string, lists: {blocked: string[]; ignored: string[]}, blockingEnabled: boolean): SnippetCover {
    if (!post || post.state === Posts.POST_DELETED) {
        return '';
    }
    if (blockingEnabled && post.user_id && post.user_id !== me) {
        if (lists.blocked.includes(post.user_id)) {
            return 'block';
        }
        if (lists.ignored.includes(post.user_id)) {
            return 'ignore';
        }
    }
    return isSpoiler(post) ? 'spoiler' : '';
}

/** getSnippetCover is coverOf for the current user. */
export function getSnippetCover(state: GlobalState, post?: CoverPost): SnippetCover {
    return coverOf(post, getCurrentUserId(state), getBlockLists(state), isUserBlockingEnabled(state));
}

/**
 * useSnippetCover gives a function saying, for a message shown as a snippet, the placeholder to show instead of its
 * text ("Spoiler", "Message from someone you ignore"), or '' when its text can show.
 */
export function useSnippetCover(): (post?: CoverPost) => string {
    const {formatMessage} = useIntl();
    const me = useSelector(getCurrentUserId);
    const lists = useSelector(getBlockLists);
    const enabled = useSelector(isUserBlockingEnabled);
    return useCallback((post?: CoverPost) => {
        switch (coverOf(post, me, lists, enabled)) {
        case 'spoiler':
            return formatMessage({id: 'fusion.home.spoiler', defaultMessage: 'Spoiler'});
        case 'block':
            return formatMessage({id: 'fusion.folded.blocked', defaultMessage: 'Message from someone you blocked'});
        case 'ignore':
            return formatMessage({id: 'fusion.folded.ignored', defaultMessage: 'Message from someone you ignore'});
        default:
            return '';
        }
    }, [formatMessage, me, lists, enabled]);
}
