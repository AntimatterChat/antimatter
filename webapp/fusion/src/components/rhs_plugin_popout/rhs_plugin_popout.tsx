// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useMemo, useState} from 'react';
import {defineMessage} from 'react-intl';
import {useSelector} from 'react-redux';
import {useLocation, useParams} from 'react-router-dom';

import LoadingScreen from 'components/loading_screen';

import AppPanelHeader from 'fusion/rhs/app_panel_header';
import Pluggable from 'plugins/pluggable';
import {readPanelState, withPanelState} from 'plugins/rhs_plugin/panel_state';
import usePopoutTitle from 'utils/popouts/use_popout_title';

import type {GlobalState} from 'types/store';

export const RHS_PLUGIN_TITLE = defineMessage({
    id: 'rhs_plugin_popout.title',
    // eslint-disable-next-line formatjs/enforce-placeholders -- provided later
    defaultMessage: '{pluginDisplayName} - {serverName}',
});

export default function RhsPluginPopout() {
    const {pluginId} = useParams<{pluginId: string}>();

    // Where the panel was when it was popped out (?state=), read once: the app starts there. As it moves on, the
    // window's address follows it, so a reload starts where the person is (the other parameters stay).
    const location = useLocation();
    const [initialPanelState] = useState(() => readPanelState(location.search));
    const setPanelState = useCallback((state: string | null) => {
        const search = withPanelState(window.location.search, state);
        if (search !== window.location.search) {
            window.history.replaceState(window.history.state, '', `${window.location.pathname}${search}${window.location.hash}`);
        }
    }, []);
    const pluginDisplayName = useSelector((state: GlobalState) => (pluginId ? state.plugins.plugins[pluginId]?.name : undefined));
    usePopoutTitle(RHS_PLUGIN_TITLE, {pluginDisplayName: pluginDisplayName ?? pluginId});

    const pluginComponentData = useSelector((state: GlobalState) => {
        const rhsPlugins = state.plugins.components.RightHandSidebarComponent;
        return rhsPlugins.find((element) => element.pluginId === pluginId);
    });

    const {showPluggable, pluggableId, title} = useMemo(() => {
        const pluginTitle = pluginComponentData ? pluginComponentData.title : '';
        const componentId = pluginComponentData ? pluginComponentData.id : '';

        return {
            showPluggable: Boolean(pluginComponentData),
            pluggableId: componentId,
            title: pluginTitle,
        };
    }, [pluginComponentData]);

    if (!showPluggable) {
        return <LoadingScreen/>;
    }

    // The panel's header in its own window: the app's icon and name, without the buttons of the main window's
    // panel (its size is the window's, and the window has its own close button).
    return (
        <>
            <AppPanelHeader
                pluginId={pluginId}
                title={title}
            />
            <Pluggable
                pluggableName='RightHandSidebarComponent'
                pluggableId={pluggableId}
                initialPanelState={initialPanelState}
                setPanelState={setPanelState}
            />
        </>
    );
}

