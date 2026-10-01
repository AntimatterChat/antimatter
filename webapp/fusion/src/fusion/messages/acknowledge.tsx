// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {acknowledgePost, unacknowledgePost} from 'mattermost-redux/actions/posts';
import {isPostAcknowledgementsEnabled} from 'mattermost-redux/selectors/entities/posts';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

// Acknowledge is the button under a message that requests acknowledgement.
export default function Acknowledge({post}: {post: Post}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const enabled = useSelector(isPostAcknowledgementsEnabled);
    const me = useSelector(getCurrentUserId);

    if (!enabled || !post.metadata?.priority?.requested_ack) {
        return null;
    }
    const acks = (post.metadata.acknowledgements || []).filter((a) => a.acknowledged_at > 0);
    const mine = acks.some((a) => a.user_id === me);
    const own = post.user_id === me;

    return (
        <div>
            <button
                className={am('ack', {mine})}
                aria-pressed={mine}
                disabled={own}
                onClick={() => dispatch(mine ? unacknowledgePost(post.id) : acknowledgePost(post.id))}
            >
                <Icon
                    name='check'
                    size='xs'
                />
                {mine ? formatMessage({id: 'fusion.ack.done', defaultMessage: 'Acknowledged'}) : formatMessage({id: 'fusion.ack.do', defaultMessage: 'Acknowledge'})}
                {acks.length > 0 && (
                    <>
                        {' '}
                        <span className={am('faces')}>
                            {acks.slice(0, 3).map((a) => (
                                <Avatar
                                    key={a.user_id}
                                    userId={a.user_id}
                                    size='xs'
                                />
                            ))}
                        </span>
                        {acks.length}
                    </>
                )}
            </button>
        </div>
    );
}
