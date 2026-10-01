// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {Posts} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getPost} from 'mattermost-redux/selectors/entities/posts';
import {isCollapsedThreadsEnabled} from 'mattermost-redux/selectors/entities/preferences';
import type {UserMentionKey} from 'mattermost-redux/selectors/entities/users';
import {getCurrentUserId, getCurrentUserMentionKeys} from 'mattermost-redux/selectors/entities/users';

import {isSystemMessage} from 'utils/post_utils';

import type {GlobalState} from 'types/store';

const escape = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

// mentions tells whether a message's text contains one of your mention keys (@you, @here, @channel, @all, your
// first name or your own keywords, as set in your notification settings), as a whole word.
export function mentions(message: string, keys: UserMentionKey[]): boolean {
    if (!message) {
        return false;
    }

    // Code isn't read for mentions.
    const text = message.replace(/```[\s\S]*?```/g, ' ').replace(/`[^`\n]*`/g, ' ');
    return keys.some(({key, caseSensitive}) => {
        if (!key) {
            return false;
        }
        const re = new RegExp(`(^|[^\\w@.-])${escape(key)}(?=$|[^\\w.-]|\\.(?:\\s|$))`, caseSensitive ? '' : 'i');
        return re.test(text);
    });
}

// useConcernsMe tells whether a message concerns you, for the mockup's .hl-me highlight: someone else's message that
// mentions you or replies inline to your message, or, when replies show in the channel, a reply to your message.
export function useConcernsMe(post: Post | undefined, inThread: boolean): boolean {
    return useSelector((state: GlobalState) => {
        if (!post || post.state === Posts.POST_DELETED || isSystemMessage(post)) {
            return false;
        }
        const me = getCurrentUserId(state);
        if (post.user_id === me) {
            return false;
        }
        if (mentions(post.message, getCurrentUserMentionKeys(state))) {
            return true;
        }
        const quotedId = post.props?.reply_to;
        if (typeof quotedId === 'string' && quotedId && getConfig(state).EnableInlineReplies !== 'false') {
            const quoted = getPost(state, quotedId);
            const quotedAuthor = quoted ? quoted.user_id : post.metadata?.reply_to?.user_id;
            if (quotedAuthor === me) {
                return true;
            }
        }
        if (post.root_id && !inThread && !isCollapsedThreadsEnabled(state)) {
            return getPost(state, post.root_id)?.user_id === me;
        }
        return false;
    });
}
