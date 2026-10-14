// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {Dialog} from 'fusion/components/layer';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

import {BLOCK_PROMPT_CATEGORY, setBlockLevel} from './blocking';

// BlockPromptDialog offers to block someone whose message requests you keep ignoring, as Discord does: block them,
// not now, or never ask again about them.
export default function BlockPromptDialog({userId, onClose}: {userId: string; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const me = useSelector(getCurrentUserId);
    const name = useDisplayName(useUser(userId));
    const [busy, setBusy] = useState(false);

    const block = async () => {
        setBusy(true);
        const result = await dispatch(setBlockLevel(userId, 'block'));
        setBusy(false);
        onClose();
        if (!('error' in result && result.error)) {
            toast(formatMessage({id: 'fusion.block.blocked', defaultMessage: '{name} is blocked'}, {name}));
        }
    };
    const neverAsk = () => {
        dispatch(savePreferences(me, [{user_id: me, category: BLOCK_PROMPT_CATEGORY, name: userId, value: 'off'}]));
        onClose();
    };

    const title = formatMessage({id: 'fusion.blockPrompt.title', defaultMessage: 'Block {name}?'}, {name});
    return (
        <Dialog
            label={title}
            onClose={onClose}
        >
            <div className={am('pane')}>
                <h2>{title}</h2>
                <p className={am('lead')}>
                    {formatMessage({id: 'fusion.blockPrompt.text', defaultMessage: 'You keep ignoring {name}\'s messages. Block them? They won\'t be able to message you any more, and they won\'t be told.'}, {name})}
                </p>
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={neverAsk}
                    >
                        {formatMessage({id: 'fusion.blockPrompt.never', defaultMessage: 'Don\'t ask again for this person'})}
                    </button>
                    <span className={am('grow')}/>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.blockPrompt.notNow', defaultMessage: 'Not now'})}
                    </button>
                    <button
                        type='button'
                        className={am('btn', 'danger')}
                        disabled={busy}
                        onClick={block}
                    >
                        {formatMessage({id: 'fusion.blockPrompt.block', defaultMessage: 'Block'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
