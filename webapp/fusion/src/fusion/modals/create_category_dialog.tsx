// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {createCategory} from 'mattermost-redux/actions/channel_categories';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';

import {Dialog} from 'fusion/components/layer';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

// CreateCategoryDialog makes a sidebar category: the mockup's "Create category".
export default function CreateCategoryDialog({onClose}: {onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const teamId = useSelector(getCurrentTeamId);
    const [name, setName] = useState('');
    const [error, setError] = useState('');
    const inputRef = useRef<HTMLInputElement>(null);

    useEffect(() => {
        inputRef.current?.focus();
    }, []);

    const create = async () => {
        const displayName = name.trim();
        if (!displayName || !teamId) {
            return;
        }
        const result = await dispatch(createCategory(teamId, displayName));
        if (result && 'error' in result && result.error) {
            setError((result.error as {message?: string}).message || formatMessage({id: 'fusion.category.failed', defaultMessage: 'The category could not be created.'}));
            return;
        }
        toast(formatMessage({id: 'fusion.toast.categoryCreated', defaultMessage: 'Category “{name}” created'}, {name: displayName}));
        onClose();
    };

    const title = formatMessage({id: 'fusion.category.title', defaultMessage: 'Create category'});
    return (
        <Dialog
            label={title}
            onClose={onClose}
            onSubmit={create}
        >
            <div className={am('pane')}>
                <h2>{title}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.category.lead', defaultMessage: 'Group channels in the sidebar. Only you see your categories.'})}</p>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.category.name', defaultMessage: 'Name'})}</span>
                    <input
                        ref={inputRef}
                        type='text'
                        value={name}
                        maxLength={22}
                        required={true}
                        placeholder={formatMessage({id: 'fusion.category.placeholder', defaultMessage: 'e.g. Experiments'})}
                        onChange={(e) => setName(e.target.value)}
                    />
                </label>
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
                        {formatMessage({id: 'fusion.category.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button
                        className={am('btn', 'primary')}
                        disabled={!name.trim()}
                    >
                        {title}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
