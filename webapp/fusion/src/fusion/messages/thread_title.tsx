// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {editPost} from 'actions/views/posts';

import {Dialog} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

// A topic is a named conversation inside its channel (Antimatter's name for what was a "named thread"): a thread, the
// stream of replies to a message, becomes a topic once it has a name. The name is kept in its first message's props
// (the data keeps its old "thread title" name), it shows in the thread's header, and the topics you follow stay
// listed under their channel in the sidebar.
export const THREAD_TITLE_PROP = 'am_thread_title';

export function getThreadTitle(post?: Pick<Post, 'props'>): string {
    const title = post?.props?.[THREAD_TITLE_PROP];
    return typeof title === 'string' ? title.trim() : '';
}

// TopicDialog makes a thread a topic by naming it, renames a topic, or turns it back into a plain thread when its
// name is left empty.
export function TopicDialog({post, onClose}: {post: Post; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const current = getThreadTitle(post);
    const [name, setName] = useState(current);
    const title = current ? formatMessage({id: 'fusion.topic.renameTitle', defaultMessage: 'Rename the topic'}) : formatMessage({id: 'fusion.topic.makeTitle', defaultMessage: 'Make it a topic'});
    return (
        <Dialog
            label={title}
            onClose={onClose}
            onSubmit={() => {
                dispatch(editPost({id: post.id, props: {...post.props, [THREAD_TITLE_PROP]: name.trim()}} as unknown as Post));
                onClose();
            }}
        >
            <div className={am('pane')}>
                <h2>{title}</h2>
                <p className={am('lead')}>
                    {formatMessage({id: 'fusion.topic.lead', defaultMessage: 'A topic is a named conversation inside its channel: everyone who follows it sees it under the channel in their sidebar.'})}
                    {current ? ' ' + formatMessage({id: 'fusion.topic.leadRename', defaultMessage: 'Leave the name empty to make it a plain thread again.'}) : ''}
                </p>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.thread.name', defaultMessage: 'Name'})}</span>
                    <input
                        autoFocus={true}
                        value={name}
                        maxLength={60}
                        placeholder={formatMessage({id: 'fusion.thread.namePlaceholder', defaultMessage: 'e.g. Release 2.1'})}
                        onChange={(e) => setName(e.target.value)}
                    />
                </label>
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.settings.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button className={am('btn', 'primary')}>{formatMessage({id: 'fusion.settings.save', defaultMessage: 'Save'})}</button>
                </div>
            </div>
        </Dialog>
    );
}
