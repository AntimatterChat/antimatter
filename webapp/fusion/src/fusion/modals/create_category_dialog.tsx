// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {createCategory} from 'mattermost-redux/actions/channel_categories';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';

import {Dialog} from 'fusion/components/layer';
import {Toggle} from 'fusion/modals/settings_controls';
import {useToast} from 'fusion/shell/toast_context';
import {addServerCategory, isTeamAdmin, renameServerCategory} from 'fusion/sidebar/server_categories';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

type Props = {
    onClose: () => void;

    // The server category to rename, if renaming one.
    renaming?: string;
};

// CreateCategoryDialog makes a sidebar category: the mockup's "Create category". A team admin can make it for everyone
// (a server category), open or locked, and rename server categories here.
export default function CreateCategoryDialog({onClose, renaming}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const teamId = useSelector(getCurrentTeamId);
    const admin = useSelector((state: GlobalState) => isTeamAdmin(state, teamId));
    const [name, setName] = useState(renaming || '');
    const [forEveryone, setForEveryone] = useState(false);
    const [locked, setLocked] = useState(false);
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
        let result;
        if (renaming) {
            result = await dispatch(renameServerCategory(teamId, renaming, displayName));
        } else if (forEveryone) {
            result = await dispatch(addServerCategory(teamId, displayName, locked));
        } else {
            result = await dispatch(createCategory(teamId, displayName));
        }
        if (result && 'error' in result && result.error) {
            setError((result.error as {message?: string}).message || formatMessage({id: 'fusion.category.failed', defaultMessage: 'The category could not be created.'}));
            return;
        }
        if (renaming) {
            toast(formatMessage({id: 'fusion.toast.categoryRenamed', defaultMessage: 'Renamed to “{name}” for everyone'}, {name: displayName}));
        } else {
            toast(forEveryone ? formatMessage({id: 'fusion.toast.serverCategoryCreated', defaultMessage: 'Category “{name}” created for everyone'}, {name: displayName}) : formatMessage({id: 'fusion.toast.categoryCreated', defaultMessage: 'Category “{name}” created'}, {name: displayName}));
        }
        onClose();
    };

    const title = renaming ? formatMessage({id: 'fusion.category.renameTitle', defaultMessage: 'Rename category'}) : formatMessage({id: 'fusion.category.title', defaultMessage: 'Create category'});
    let lead = formatMessage({id: 'fusion.category.lead', defaultMessage: 'Group channels in the sidebar. Only you see your categories.'});
    if (renaming) {
        lead = formatMessage({id: 'fusion.category.renameLead', defaultMessage: 'Everyone sees the new name; its channels stay in it.'});
    } else if (forEveryone) {
        lead = formatMessage({id: 'fusion.category.everyoneLead', defaultMessage: 'Everyone in the team sees it above their own categories. Members put their channels in it unless it\'s locked.'});
    }
    return (
        <Dialog
            label={title}
            onClose={onClose}
            onSubmit={create}
        >
            <div className={am('pane')}>
                <h2>{title}</h2>
                <p className={am('lead')}>{lead}</p>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.category.name', defaultMessage: 'Name'})}</span>
                    <input
                        ref={inputRef}
                        type='text'
                        value={name}
                        maxLength={forEveryone || renaming ? 64 : 22}
                        required={true}
                        placeholder={formatMessage({id: 'fusion.category.placeholder', defaultMessage: 'e.g. Experiments'})}
                        onChange={(e) => setName(e.target.value)}
                    />
                </label>
                {admin && !renaming && (
                    <Toggle
                        title={formatMessage({id: 'fusion.category.forEveryone', defaultMessage: 'For everyone'})}
                        desc={formatMessage({id: 'fusion.category.forEveryoneDesc', defaultMessage: 'A category of the team, set up by its admins, rather than one of yours.'})}
                        on={forEveryone}
                        onChange={setForEveryone}
                    />
                )}
                {admin && !renaming && forEveryone && (
                    <Toggle
                        title={formatMessage({id: 'fusion.category.locked', defaultMessage: 'Locked'})}
                        desc={formatMessage({id: 'fusion.category.lockedDesc', defaultMessage: 'Only admins put channels in it, and its channels stay there for everyone.'})}
                        on={locked}
                        onChange={setLocked}
                    />
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
                        {formatMessage({id: 'fusion.category.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button
                        className={am('btn', 'primary')}
                        disabled={!name.trim()}
                    >
                        {renaming ? formatMessage({id: 'fusion.category.rename', defaultMessage: 'Rename'}) : title}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
