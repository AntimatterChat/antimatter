// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch} from 'react-redux';

import type {Post} from '@mattermost/types/posts';
import type {UserProfile} from '@mattermost/types/users';

import {selectPost} from 'actions/views/rhs';

import Avatar from 'fusion/components/avatar';
import {am} from 'fusion/utils/class_names';

import {useWhen} from './time';

// ThreadSummary sits under a message with replies: who replied, how many, when, and opens the thread.
export default function ThreadSummary({post}: {post: Post}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const when = useWhen();
    const replies = post.reply_count || 0;
    const participants = (post.participants || []).slice(0, 3);

    // Like the classic thread footer: only for threads with replies (a call post is followed by its starter)
    if (!replies) {
        return null;
    }
    return (
        <button
            className={am('thread-sum')}
            onClick={() => dispatch(selectPost(post))}
        >
            {participants.length > 0 && (
                <span className={am('faces')}>
                    {participants.map((u: UserProfile) => (
                        <Avatar
                            key={u.id}
                            userId={u.id}
                            size='sm'
                        />
                    ))}
                </span>
            )}
            <span className={am('count')}>
                {formatMessage({id: 'fusion.thread.replies', defaultMessage: '{count, plural, one {# reply} other {# replies}}'}, {count: replies})}
            </span>
            {post.last_reply_at ? (
                <span>{formatMessage({id: 'fusion.thread.lastReply', defaultMessage: 'Last reply {when}'}, {when: when(post.last_reply_at)})}</span>
            ) : null}
            {post.is_following && (
                <span style={{color: 'var(--am-anti)'}}>{formatMessage({id: 'fusion.thread.following', defaultMessage: 'Following'})}</span>
            )}
        </button>
    );
}
