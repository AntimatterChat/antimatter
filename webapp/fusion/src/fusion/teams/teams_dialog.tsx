// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import {Dialog} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

import TeamsSettings from './teams_settings';

// TeamsDialog is the community's teams for those who manage nothing in it (the community menu's Teams…): the community
// settings' Teams tab on its own.
export default function TeamsDialog({onExited}: {onExited: () => void}) {
    const {formatMessage} = useIntl();
    return (
        <Dialog
            label={formatMessage({id: 'fusion.teamSettings.teams', defaultMessage: 'Teams'})}
            onClose={onExited}
            small={false}
        >
            <div className={am('pane', 'roles-pane')}>
                <h2>{formatMessage({id: 'fusion.teamSettings.teams', defaultMessage: 'Teams'})}</h2>
                <TeamsSettings/>
            </div>
        </Dialog>
    );
}
