// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {editPost} from 'actions/views/posts';

import {Dialog} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

// A named thread is a sub-chat of its channel: its name is kept in its first message's props, it shows in the
// thread's header, and the named threads you follow stay listed under their channel in the sidebar.
export const THREAD_TITLE_PROP = 'am_thread_title';

export function getThreadTitle(post?: Pick<Post, 'props'>): string {
    const title = post?.props?.[THREAD_TITLE_PROP];
    return typeof title === 'string' ? title.trim() : '';
}

// NameThreadDialog names a thread, or takes its name away when left empty.
export function NameThreadDialog({post, onClose}: {post: Post; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const [name, setName] = useState(getThreadTitle(post));
    const title = formatMessage({id: 'fusion.thread.nameTitle', defaultMessage: 'Name this thread'});
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
                <p className={am('lead')}>{formatMessage({id: 'fusion.thread.nameLead', defaultMessage: 'A named thread is a sub-chat of its channel: its followers see it under the channel in their sidebar.'})}</p>
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
