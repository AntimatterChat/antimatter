// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {unsetEditingPost} from 'actions/post_actions';
import {editPost} from 'actions/views/posts';

import DeletePostModal from 'components/delete_post_modal';

import {am} from 'fusion/utils/class_names';
import {openDialog} from 'fusion/utils/modals';
import {ModalIdentifiers} from 'utils/constants';

type Props = {
    post: Post;
    inThread: boolean;
};

// EditForm replaces a message's body while it's being edited: the mockup's .edit-form. Enter saves, Shift+Enter
// adds a line and Escape cancels; saving an empty message offers to delete it, like the classic web app.
export default function EditForm({post, inThread}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const [value, setValue] = useState(post.message);
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState('');
    const ref = useRef<HTMLTextAreaElement>(null);

    useEffect(() => {
        const el = ref.current;
        if (el) {
            el.focus();
            el.setSelectionRange(el.value.length, el.value.length);
            el.style.height = Math.min(el.scrollHeight + 2, 220) + 'px';
        }
    }, []);

    const cancel = () => dispatch(unsetEditingPost());

    const save = async () => {
        if (saving) {
            return;
        }
        const message = value.trim();
        if (!message && !post.file_ids?.length) {
            dispatch(unsetEditingPost());
            dispatch(openDialog(ModalIdentifiers.DELETE_POST, DeletePostModal, {post, isRHS: inThread}));
            return;
        }
        if (message === post.message.trim()) {
            cancel();
            return;
        }
        setSaving(true);
        const result = await dispatch(editPost({...post, message}));
        setSaving(false);
        if (result && 'error' in result && result.error) {
            setError(result.error.message || formatMessage({id: 'fusion.edit.failed', defaultMessage: 'The message could not be saved.'}));
            return;
        }
        cancel();
    };

    return (
        <form
            className={am('edit-form')}
            onSubmit={(e) => {
                e.preventDefault();
                save();
            }}
        >
            <textarea
                ref={ref}
                value={value}
                aria-label={formatMessage({id: 'fusion.edit.label', defaultMessage: 'Edit message'})}
                onChange={(e) => setValue(e.target.value)}
                onKeyDown={(e) => {
                    if (e.key === 'Enter' && !e.shiftKey && !e.nativeEvent.isComposing) {
                        e.preventDefault();
                        save();
                    } else if (e.key === 'Escape') {
                        e.preventDefault();
                        e.stopPropagation();
                        cancel();
                    }
                }}
            />
            <div className={am('edit-actions')}>
                <span
                    className={am('grow')}
                    role={error ? 'alert' : undefined}
                >
                    {error || formatMessage({id: 'fusion.edit.hint', defaultMessage: 'Enter to save · Shift+Enter for a new line · Esc to cancel'})}
                </span>
                <button
                    type='button'
                    className={am('btn')}
                    onClick={cancel}
                >
                    {formatMessage({id: 'fusion.edit.cancel', defaultMessage: 'Cancel'})}
                </button>
                <button
                    className={am('btn', 'primary')}
                    disabled={saving}
                >
                    {formatMessage({id: 'fusion.edit.save', defaultMessage: 'Save'})}
                </button>
            </div>
        </form>
    );
}
