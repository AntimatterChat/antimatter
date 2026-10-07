// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {removePost} from 'mattermost-redux/actions/posts';

import {createPost} from 'actions/post_actions';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

// FailedPost says that a message couldn't be sent, under it, with what the classic web app offers: send it again, or
// drop it.
export default function FailedPost({post}: {post: Post}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();

    const retry = () => {
        const again = {...post};
        Reflect.deleteProperty(again, 'id');
        dispatch(createPost(again, []));
    };

    return (
        <div
            className={am('failed-bar')}
            role='alert'
        >
            <Icon
                name='x'
                size='xs'
            />
            <span>{formatMessage({id: 'fusion.message.failed', defaultMessage: 'Not sent'})}</span>
            <button
                type='button'
                onClick={retry}
            >
                {formatMessage({id: 'fusion.message.retry', defaultMessage: 'Retry'})}
            </button>
            <button
                type='button'
                onClick={() => dispatch(removePost(post))}
            >
                {formatMessage({id: 'fusion.message.cancelFailed', defaultMessage: 'Cancel'})}
            </button>
        </div>
    );
}
