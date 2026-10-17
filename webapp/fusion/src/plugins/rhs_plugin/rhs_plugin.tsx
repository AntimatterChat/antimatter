// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useRef} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {getCurrentChannel} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {closeRightHandSide} from 'actions/views/rhs';

import {RHS_PLUGIN_TITLE} from 'components/rhs_plugin_popout/rhs_plugin_popout';

import AppPanelHeader from 'fusion/rhs/app_panel_header';
import Pluggable from 'plugins/pluggable';
import {isChannelPopoutWindow, isPopoutWindow, popoutRhsPlugin} from 'utils/popouts/popout_windows';

import type {GlobalState} from 'types/store';

import {normalizePanelState} from './panel_state';

export type Props = {
    showPluggable: boolean;
    pluggableId: string;
    title: React.ReactNode;
    pluginId?: string;
    showPopout?: boolean;

    // Whether the person resized the panel away from its default width (Fusion's right-hand slot), and how to give
    // it back: the header's "Restore the panel's size" button.
    sized?: boolean;
    onRestoreSize?: () => void;
};

// RhsPlugin is an app's side panel: the header every app's panel has (its icon and name, then restore its size, open
// it in a new window and close it: AppPanelHeader), over the app's own panel, which draws its controls itself.
const RhsPlugin = ({showPluggable, pluggableId, title, pluginId, showPopout = true, sized = false, onRestoreSize}: Props) => {
    const intl = useIntl();
    const dispatch = useDispatch();
    const currentTeam = useSelector(getCurrentTeam);
    const currentChannel = useSelector(getCurrentChannel);
    const pluginDisplayName = useSelector((state: GlobalState) => (pluginId ? state.plugins.plugins[pluginId]?.name : undefined));

    // Where the app's panel says it is (its setPanelState prop), for its popout to start there. Forgotten when the
    // panel goes, or another app's takes its place: the cleanup runs before the next panel's effects report again.
    const panelState = useRef<string | undefined>(undefined);
    const setPanelState = useCallback((state: string | null) => {
        panelState.current = normalizePanelState(state);
    }, []);
    useEffect(() => () => {
        panelState.current = undefined;
    }, [pluggableId]);

    const newWindowHandler = useCallback(() => {
        if (pluginId && currentTeam && currentChannel) {
            popoutRhsPlugin(
                intl.formatMessage(
                    RHS_PLUGIN_TITLE,
                    {
                        pluginDisplayName: pluginDisplayName ?? pluginId,
                        serverName: '{serverName}',
                    },
                ),
                pluginId,
                currentTeam.name,
                currentChannel.name,
                panelState.current,
            );
        }
    }, [intl, pluginId, currentTeam, currentChannel, pluginDisplayName]);

    return (
        <div
            id='rhsContainer'
            className='sidebar-right__body'
        >
            {pluginId && (
                <AppPanelHeader
                    pluginId={pluginId}
                    title={title}
                    sized={sized}
                    onRestoreSize={onRestoreSize}
                    onNewWindow={showPopout ? newWindowHandler : undefined}
                    onClose={!isPopoutWindow() || isChannelPopoutWindow() ? () => dispatch(closeRightHandSide()) : undefined}
                />
            )}
            {
                showPluggable &&
                <Pluggable
                    pluggableName='RightHandSidebarComponent'
                    pluggableId={pluggableId}
                    setPanelState={setPanelState}
                />
            }
        </div>
    );
};

export default React.memo(RhsPlugin);
