// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {closeRightHandSide} from 'actions/views/rhs';
import {getIsRhsExpanded, getIsRhsOpen, getPluggableId, getRhsState, getSelectedPostId} from 'selectors/rhs';

import ChannelInfoRhs from 'components/channel_info_rhs';
import PostEditHistory from 'components/post_edit_history';
import Search from 'components/search/index';

import {PanelAppIcon} from 'fusion/apps/app_rail';
import {getKnownApp} from 'fusion/apps/known_apps';
import {useOpenVoiceChat, VoiceChatPanel} from 'fusion/calls/voice_chat';
import Icon from 'fusion/components/icon';
import {SheetBack} from 'fusion/components/layer';
import {useIsPhone} from 'fusion/hooks/media';
import MemberList from 'fusion/members/member_list';
import ModViewPanel from 'fusion/moderation/mod_view';
import {a11yRegion} from 'fusion/utils/a11y_regions';
import {am} from 'fusion/utils/class_names';
import RhsPlugin from 'plugins/rhs_plugin';
import {getHistory} from 'utils/browser_history';
import {RHSStates} from 'utils/constants';

import type {GlobalState} from 'types/store';

import {PanelResizeHandle, RestoreWidthButton, usePanelWidth} from './panel_width';
import ResultsPanel from './results_panel';
import ThreadPanel from './thread_panel';

// A panel of the classic web app, shown as it is inside the Fusion UI's right-hand slot. Its own header (title,
// expand, popout and close buttons) is styled as the mockup's panel header.
function ClassicPanel({children}: {children: React.ReactNode}) {
    return (
        <div className={am('rhs-classic')}>
            <div className='sidebar--right__content'>{children}</div>
        </div>
    );
}

// RightPanel is the right-hand slot: a thread, results, a plugin's panel, or the member list.
export default function RightPanel() {
    const open = useSelector(getIsRhsOpen);
    const rhsState = useSelector(getRhsState);
    const selectedPostId = useSelector(getSelectedPostId);
    const expanded = useSelector(getIsRhsExpanded);
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const pluginId = useSelector((state: GlobalState) => state.plugins.components.RightHandSidebarComponent.find((c) => c.id === getPluggableId(state))?.pluginId);

    // A plugin that is also a product (Boards, Playbooks) can show its panel as its full page.
    const product = useSelector((state: GlobalState) => (pluginId ? state.plugins.components.Product?.find((p) => p.pluginId === pluginId) : undefined));
    const voiceChat = useOpenVoiceChat();
    const phone = useIsPhone();

    // An app's panel is resizable (see usePanelWidth); the other panels keep their widths.
    const panelRef = useRef<HTMLElement>(null);
    const isApp = open && !selectedPostId && rhsState === RHSStates.PLUGIN;
    const roomyApp = Boolean(pluginId && getKnownApp(pluginId)?.widePanel);
    const panelWidth = usePanelWidth(panelRef, isApp, roomyApp ? 'roomy' : 'wide');

    if (!open) {
        return voiceChat ? <VoiceChatPanel channelId={voiceChat}/> : null;
    }

    let content: React.ReactNode = null;
    let wide = false;
    let roomy = false;
    if (selectedPostId) {
        content = <ThreadPanel rootId={selectedPostId}/>;
    } else if (rhsState === RHSStates.SEARCH || rhsState === RHSStates.MENTION || rhsState === RHSStates.FLAG || rhsState === RHSStates.PIN) {
        content = <ResultsPanel rhsState={rhsState}/>;
    } else if (rhsState === RHSStates.CHANNEL_MEMBERS) {
        content = <MemberList inPanel={true}/>;
    } else if (rhsState === RHSStates.MOD_VIEW) {
        content = <ModViewPanel/>;
    } else if (rhsState === RHSStates.PLUGIN) {
        wide = true;
        roomy = roomyApp;
        content = (
            <ClassicPanel>
                {pluginId && <PanelAppIcon pluginId={pluginId}/>}
                <div className={am('rhs-tools')}>
                    <RestoreWidthButton
                        sized={panelWidth.width !== null && panelWidth.bounds !== null}
                        onRestore={panelWidth.reset}
                    />
                    {product && (
                        <button
                            type='button'
                            className={am('icon-btn', 'rhs-fullpage')}
                            title={formatMessage({id: 'fusion.rhs.fullPage', defaultMessage: 'Open as a full page'})}
                            aria-label={formatMessage({id: 'fusion.rhs.fullPage', defaultMessage: 'Open as a full page'})}
                            onClick={() => {
                                dispatch(closeRightHandSide());
                                getHistory().push(product.baseURL);
                            }}
                        >
                            <Icon
                                name='fullscreen'
                                size='sm'
                            />
                        </button>
                    )}
                </div>
                <RhsPlugin/>
            </ClassicPanel>
        );
    } else if (rhsState === RHSStates.CHANNEL_INFO) {
        content = <ClassicPanel><ChannelInfoRhs/></ClassicPanel>;
    } else if (rhsState === RHSStates.EDIT_HISTORY) {
        content = <ClassicPanel><PostEditHistory/></ClassicPanel>;
    } else if (rhsState === RHSStates.CHANNEL_FILES) {
        content = (
            <ClassicPanel>
                <Search
                    isSideBarRight={true}
                    isSideBarRightOpen={true}
                    getFocus={() => {}}
                    channelDisplayName=''
                />
            </ClassicPanel>
        );
    }

    if (!content) {
        return null;
    }

    // An app's panel has no expand button: its user drags its edge instead, and its "expanded" state only tells
    // plugins it's wider than its default. The classic expand button of the other panels toggles between its expand
    // and collapse icons on sidebar--right--expanded. On a phone every panel takes the whole screen, with a back
    // button instead of the buttons that size, pop out or close it.
    const sized = wide && panelWidth.width !== null && !phone;
    return (
        <aside
            ref={panelRef}
            {...a11yRegion(am('rhs', {wide, roomy, sized, expanded: expanded && !wide, resizing: wide && panelWidth.dragging}) + (expanded ? ' sidebar--right--expanded' : ''), 'rhs')}
            style={sized ? {'--am-rhs-w': `${panelWidth.width}px`} as React.CSSProperties : undefined}
        >
            {wide && !phone && <PanelResizeHandle panel={panelWidth}/>}
            {phone && (selectedPostId || rhsState !== RHSStates.CHANNEL_MEMBERS) && <SheetBack onBack={() => dispatch(closeRightHandSide())}/>}
            {content}
        </aside>
    );
}
