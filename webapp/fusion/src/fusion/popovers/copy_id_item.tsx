// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import {MenuItem, MenuSeparator} from 'fusion/components/menu';
import {useDisplayPrefs} from 'fusion/hooks/display';
import {useToast} from 'fusion/shell/toast_context';
import {copyToClipboard} from 'utils/utils';

// CopyIdItem is the "Copy ID" item that developer mode adds at the end of the menus of messages, channels, teams
// and people, for the API and bots.
export default function CopyIdItem({id, onDone}: {id: string; onDone: () => void}) {
    const {formatMessage} = useIntl();
    const toast = useToast();
    const {developer} = useDisplayPrefs();
    if (!developer || !id) {
        return null;
    }
    return (
        <>
            <MenuSeparator/>
            <MenuItem
                icon='copy'
                label={formatMessage({id: 'fusion.menu.copyId', defaultMessage: 'Copy ID'})}
                sub={id}
                onClick={() => {
                    copyToClipboard(id);
                    toast(formatMessage({id: 'fusion.toast.idCopied', defaultMessage: 'ID copied'}));
                    onDone();
                }}
            />
        </>
    );
}
