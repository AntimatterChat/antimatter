// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Location} from 'history';
import {connect} from 'react-redux';
import {withRouter, matchPath} from 'react-router-dom';

import type {GlobalState} from '@mattermost/types/store';

import {getCurrentChannel, getUnreadStatus} from 'mattermost-redux/selectors/entities/channels';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import UnreadsStatusHandler from './unreads_status_handler';

// hideUnreads (Fusion's focus mode): no unread or mention marks in the title, the icon and the desktop app's badge.
type Props = {location: Location; hideUnreads?: boolean};

export function mapStateToProps(state: GlobalState, {location: {pathname}, hideUnreads}: Props) {
    const config = getConfig(state);
    const currentChannel = getCurrentChannel(state);
    const currentTeammate = (currentChannel && currentChannel.teammate_id) ? currentChannel : null;
    const currentTeam = getCurrentTeam(state);

    return {
        currentChannel,
        currentTeam,
        currentTeammate,
        siteName: config.SiteName,
        unreadStatus: hideUnreads ? false : getUnreadStatus(state),
        inGlobalThreads: matchPath(pathname, {path: '/:team/threads/:threadIdentifier?'}) != null,
        inDrafts: matchPath(pathname, {path: '/:team/drafts'}) != null,
        inScheduledPosts: matchPath(pathname, {path: '/:team/scheduled_posts'}) != null,
    };
}

export default withRouter(connect(mapStateToProps)(UnreadsStatusHandler));
