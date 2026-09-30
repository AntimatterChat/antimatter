// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {closeRightHandSide} from 'actions/views/rhs';
import {getIsRhsOpen, getRhsState, getSelectedPostId} from 'selectors/rhs';

import ChannelInfoRhs from 'components/channel_info_rhs';
import PostEditHistory from 'components/post_edit_history';
import Search from 'components/search/index';

import Icon from 'fusion/components/icon';
import MemberList from 'fusion/members/member_list';
import {am} from 'fusion/utils/class_names';
import RhsPlugin from 'plugins/rhs_plugin';
import {RHSStates} from 'utils/constants';

import ResultsPanel from './results_panel';
import ThreadPanel from './thread_panel';

// A panel of the classic web app, shown as it is inside the Fusion UI's right-hand slot.
function ClassicPanel({children}: {children: React.ReactNode}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    return (
        <div className={am('rhs-classic')}>
            <button
                className={am('icon-btn', 'rhs-classic-close')}
                aria-label={formatMessage({id: 'fusion.rhs.close', defaultMessage: 'Close'})}
                onClick={() => dispatch(closeRightHandSide())}
            >
                <Icon name='x'/>
            </button>
            <div className='sidebar--right__content'>{children}</div>
        </div>
    );
}

// RightPanel is the right-hand slot: a thread, results, a plugin's panel, or the member list.
export default function RightPanel() {
    const open = useSelector(getIsRhsOpen);
    const rhsState = useSelector(getRhsState);
    const selectedPostId = useSelector(getSelectedPostId);

    if (!open) {
        return null;
    }

    let content: React.ReactNode = null;
    let wide = false;
    if (selectedPostId) {
        content = <ThreadPanel rootId={selectedPostId}/>;
    } else if (rhsState === RHSStates.SEARCH || rhsState === RHSStates.MENTION || rhsState === RHSStates.FLAG || rhsState === RHSStates.PIN) {
        content = <ResultsPanel rhsState={rhsState}/>;
    } else if (rhsState === RHSStates.CHANNEL_MEMBERS) {
        content = <MemberList inPanel={true}/>;
    } else if (rhsState === RHSStates.PLUGIN) {
        wide = true;
        content = <ClassicPanel><RhsPlugin/></ClassicPanel>;
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
    return <aside className={am('rhs', {wide})}>{content}</aside>;
}
