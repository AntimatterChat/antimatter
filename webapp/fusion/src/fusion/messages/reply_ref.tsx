// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {getPost} from 'mattermost-redux/selectors/entities/posts';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {getUser} from 'mattermost-redux/selectors/entities/users';

import {selectPost} from 'actions/views/rhs';

import Icon from 'fusion/components/icon';
import {useDisplayName} from 'fusion/hooks/users';
import {am} from 'fusion/utils/class_names';
import {permalinkPath} from 'fusion/utils/paths';
import {plainText} from 'fusion/utils/plain_text';
import {getHistory} from 'utils/browser_history';

import type {GlobalState} from 'types/store';

// ReplyRef is the "↩ Replying to Name snippet · Thread" line above a reply shown in the channel (collapsed reply
// threads off): the mockup's .reply-ref, where the classic web app said "Commented on".
export default function ReplyRef({post}: {post: Post}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const team = useSelector(getCurrentTeam);
    const root = useSelector((state: GlobalState) => getPost(state, post.root_id));
    const author = useSelector((state: GlobalState) => (root ? getUser(state, root.user_id) : undefined));
    const name = useDisplayName(author);
    const who = (root?.props?.override_username as string | undefined) || name;

    return (
        <div className={am('reply-ref')}>
            <button
                className={am('ref-jump')}
                title={formatMessage({id: 'fusion.replyRef.jump', defaultMessage: 'Jump to the original message'})}
                onClick={() => team && getHistory().push(permalinkPath(team.name, post.root_id))}
            >
                <Icon
                    name='reply'
                    size='xs'
                />
                <span>{formatMessage({id: 'fusion.replyRef.replying', defaultMessage: 'Replying to'})}</span>
                {root && <b>{who}</b>}
                <span className={am('snip')}>{root ? plainText(root.message, 160) : formatMessage({id: 'fusion.replyRef.message', defaultMessage: 'a message'})}</span>
            </button>
            <button
                className={am('ref-thread')}
                title={formatMessage({id: 'fusion.replyRef.threadTitle', defaultMessage: 'Open the conversation as a thread'})}
                aria-label={formatMessage({id: 'fusion.replyRef.threadTitle', defaultMessage: 'Open the conversation as a thread'})}
                onClick={() => dispatch(selectPost(post))}
            >
                <Icon
                    name='thread'
                    size='xs'
                />
                <span>{formatMessage({id: 'fusion.replyRef.thread', defaultMessage: 'Thread'})}</span>
            </button>
        </div>
    );
}
