// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {deleteAndRemovePost} from 'actions/post_actions';

import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import {plainText} from 'fusion/utils/plain_text';
import {getHistory} from 'utils/browser_history';

type Props = {
    post: Post;
    onClose: () => void;
};

// DeleteMessageDialog confirms deleting a message: the mockup's "Delete message?" with a preview of it.
export default function DeleteMessageDialog({post, onClose}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const [error, setError] = useState('');
    const replies = post.root_id ? 0 : post.reply_count || 0;

    const remove = async () => {
        const result = await dispatch(deleteAndRemovePost(post));
        if (result && 'error' in result && result.error) {
            setError((result.error as {message?: string}).message || formatMessage({id: 'fusion.delete.failed', defaultMessage: 'The message could not be deleted.'}));
            return;
        }

        // A permalink to the deleted message falls back to its conversation.
        const path = getHistory().location?.pathname || '';
        if (path.endsWith('/' + post.id)) {
            getHistory().replace(path.split('/').slice(0, -1).join('/'));
        }
        toast(formatMessage({id: 'fusion.toast.deleted', defaultMessage: 'Message deleted'}));
        onClose();
    };

    const title = formatMessage({id: 'fusion.delete.title', defaultMessage: 'Delete message?'});
    const preview = plainText(post.message, 180) || (post.file_ids?.length ? formatMessage({id: 'fusion.delete.files', defaultMessage: '{count, plural, one {# file} other {# files}}'}, {count: post.file_ids.length}) : '');
    return (
        <Dialog
            label={title}
            onClose={onClose}
            onSubmit={remove}
        >
            <div className={am('pane')}>
                <h2>{title}</h2>
                <p className={am('lead')}>
                    {formatMessage({id: 'fusion.delete.lead', defaultMessage: "This removes it for everyone. It can't be undone."})}
                    {replies > 0 && ' ' + formatMessage({id: 'fusion.delete.replies', defaultMessage: 'Its {count, plural, one {reply goes} other {# replies go}} with it.'}, {count: replies})}
                </p>
                {preview && (
                    <div
                        className={am('note-box')}
                        style={{background: 'var(--am-side)'}}
                    >
                        <span>{preview}</span>
                    </div>
                )}
                {error && (
                    <p
                        className={am('lead')}
                        role='alert'
                        style={{color: 'var(--am-danger)'}}
                    >
                        {error}
                    </p>
                )}
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.delete.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button
                        className={am('btn', 'danger')}
                        autoFocus={true}
                    >
                        <Icon
                            name='trash'
                            size='sm'
                        />
                        {formatMessage({id: 'fusion.delete.delete', defaultMessage: 'Delete'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
