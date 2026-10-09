// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import {Client4} from 'mattermost-redux/client';

import {Dialog} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

import {OverridesEditor, useOverrideList} from './overrides_editor';

// CategoryPermissionsDialog is what roles or people may do in the channels of a category for everyone, as Discord's
// category permissions: its channels inherit it, and what a channel sets for itself wins.
export default function CategoryPermissionsDialog({teamId, name, onClose}: {teamId: string; name: string; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const key = name.trim().toLowerCase();
    const list = useOverrideList(
        () => Client4.getCategoryOverrides(teamId).then((all) => all.filter((o) => o.category === key)),
        (o) => Client4.saveCategoryOverride(teamId, {category: name, target_type: o.target_type, target_id: o.target_id, allow: o.allow, deny: o.deny}),
        [teamId, key],
    );

    return (
        <Dialog
            label={formatMessage({id: 'fusion.categoryOverrides.title', defaultMessage: 'Permissions of {category}'}, {category: name})}
            onClose={onClose}
            small={false}
            className={am('wide')}
        >
            <div className={am('pane', 'roles-pane')}>
                <h2>{formatMessage({id: 'fusion.categoryOverrides.title', defaultMessage: 'Permissions of {category}'}, {category: name})}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.categoryOverrides.lead', defaultMessage: 'What roles and people may do in the channels of this category, on top of, or less than, what they have elsewhere. A channel can still set its own, which wins.'})}</p>
                {list.loadFailed && (
                    <p
                        className={am('lead')}
                        role='alert'
                    >
                        {formatMessage({id: 'fusion.categoryOverrides.loadFailed', defaultMessage: 'The category\'s permissions couldn\'t be loaded. Close this and try again.'})}
                    </p>
                )}
                <OverridesEditor
                    teamId={teamId}
                    overrides={list.overrides}
                    editable={list.editable}
                    onChange={list.change}
                    onAdd={list.add}
                />
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.overrides.done', defaultMessage: 'Done'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
