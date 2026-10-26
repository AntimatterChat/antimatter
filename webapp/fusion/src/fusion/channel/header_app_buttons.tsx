// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useSelector} from 'react-redux';

import {appBarEnabled} from 'mattermost-redux/selectors/entities/apps';

import {getChannelHeaderPluginComponents} from 'selectors/plugins';

import {AppIcon} from 'fusion/apps/app_rail';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';
import type {ChannelHeaderButtonAction} from 'types/store/plugins';

const NO_BUTTONS: ChannelHeaderButtonAction[] = [];

function headerButtons(state: GlobalState): ChannelHeaderButtonAction[] {
    return appBarEnabled(state) ? NO_BUTTONS : getChannelHeaderPluginComponents(state) || NO_BUTTONS;
}

// HeaderAppButtons are apps' header buttons (registerChannelHeaderButtonAction) when the app bar is turned off
// (the upstream DisableAppBar setting): they live in the app rail otherwise, which then shows none, as the classic web
// app moves them between its header and its app bar. One place either way.
export default function HeaderAppButtons() {
    const buttons = useSelector(headerButtons);
    if (!buttons.length) {
        return null;
    }
    return (
        <span className={am('head-plugins')}>
            {buttons.map((c) => (
                <AppIcon
                    key={c.id}
                    component={c}
                    inHeader={true}
                />
            ))}
        </span>
    );
}
