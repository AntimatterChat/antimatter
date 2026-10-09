// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';

import {Dialog} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

export type ConfirmOptions = {
    title: string;
    message: string;
    confirmLabel: string;
    danger?: boolean;

    // What confirming does; the dialog stays, busy, until it's done.
    onConfirm: () => unknown;
};

// ConfirmDialog asks before doing something that can't be undone.
export default function ConfirmDialog({title, message, confirmLabel, danger = false, onConfirm, onClose}: ConfirmOptions & {onClose: () => void}) {
    const {formatMessage} = useIntl();
    const [busy, setBusy] = useState(false);
    const confirm = async () => {
        setBusy(true);
        try {
            await onConfirm();
        } finally {
            setBusy(false);
            onClose();
        }
    };
    return (
        <Dialog
            label={title}
            onClose={onClose}
        >
            <div className={am('pane')}>
                <h2>{title}</h2>
                <p className={am('lead')}>{message}</p>
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.confirm.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button
                        type='button'
                        className={am('btn', danger ? 'danger' : 'primary')}
                        disabled={busy}
                        onClick={confirm}
                    >
                        {confirmLabel}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
