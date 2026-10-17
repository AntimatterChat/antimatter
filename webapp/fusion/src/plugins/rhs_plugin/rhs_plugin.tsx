// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useRef} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {getCurrentChannel} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {RHS_PLUGIN_TITLE} from 'components/rhs_plugin_popout/rhs_plugin_popout';
import SearchResultsHeader from 'components/search_results_header';

import Pluggable from 'plugins/pluggable';
import {popoutRhsPlugin} from 'utils/popouts/popout_windows';

import type {GlobalState} from 'types/store';

import {normalizePanelState} from './panel_state';

export type Props = {
    showPluggable: boolean;
    pluggableId: string;
    title: React.ReactNode;
    pluginId?: string;
    showPopout?: boolean;
};

const RhsPlugin = ({showPluggable, pluggableId, title, pluginId, showPopout = true}: Props) => {
    const intl = useIntl();
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
            <SearchResultsHeader newWindowHandler={showPopout ? newWindowHandler : undefined}>
                {title}
            </SearchResultsHeader>
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
