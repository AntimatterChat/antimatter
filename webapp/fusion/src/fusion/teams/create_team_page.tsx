// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useSelector} from 'react-redux';
import type {RouteComponentProps} from 'react-router-dom';

import {getTheme} from 'mattermost-redux/selectors/entities/preferences';

import {IconSprite} from 'fusion/components/icon';
import {LAYER_ID} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';
import {getHistory} from 'utils/browser_history';
import {applyTheme} from 'utils/utils';

import CreateTeamDialog from './create_team_dialog';

import 'fusion/styles/_module.scss';

// CreateTeamPage is /create_team in the Fusion UI: the create team dialog over an empty frame, for those who reach it
// by its address or have no team yet to open it from.
export default function CreateTeamPage(props: RouteComponentProps) { // eslint-disable-line @typescript-eslint/no-unused-vars
    const [layerReady, setLayerReady] = useState(false);
    const theme = useSelector(getTheme);

    // The person's theme, which the team views apply elsewhere.
    useEffect(() => {
        applyTheme(theme);
    }, [theme]);

    // The dialog shows in the layer, once the layer is there.
    useEffect(() => {
        const root = document.getElementById('root');
        root?.classList.add('am-root');
        setLayerReady(true);
        return () => root?.classList.remove('am-root');
    }, []);

    return (
        <>
            <IconSprite/>
            <div className={am('app', 'solo')}/>
            <div
                id={LAYER_ID}
                className={am('layer')}
            />
            {layerReady && <CreateTeamDialog onClose={() => getHistory().push('/')}/>}
        </>
    );
}
