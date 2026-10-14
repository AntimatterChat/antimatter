// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import type {Post} from '@mattermost/types/posts';

import Icon from 'fusion/components/icon';
import MessageTime from 'fusion/messages/time';
import {am} from 'fusion/utils/class_names';

import type {BlockLevel} from './blocking';

// FoldedMessage stands for a message of someone you ignore or block, as in Discord: one line, which shows the message
// when clicked. Who wrote it isn't said, so that an ignored person's name doesn't keep showing up.
export default function FoldedMessage({post, level, onShow}: {post: Post; level: Exclude<BlockLevel, ''>; onShow: () => void}) {
    const {formatMessage} = useIntl();
    return (
        <div
            className={am('msg', 'sysline', 'folded')}
            id={`post_${post.id}`}
        >
            <span className={am('sys-ic')}><Icon name={level === 'block' ? 'ban' : 'eye-off'}/></span>
            <span className={am('body')}>
                {level === 'block' ? formatMessage({id: 'fusion.folded.blocked', defaultMessage: 'Message from someone you blocked'}) : formatMessage({id: 'fusion.folded.ignored', defaultMessage: 'Message from someone you ignore'})}
                {' · '}
                <button
                    type='button'
                    className={am('folded-show')}
                    onClick={onShow}
                >
                    {formatMessage({id: 'fusion.folded.show', defaultMessage: 'Show'})}
                </button>
            </span>
            <MessageTime timestamp={post.create_at}/>
        </div>
    );
}
