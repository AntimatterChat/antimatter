// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {connect} from 'react-redux';

import {getConfig} from 'mattermost-redux/selectors/entities/general';

import type {GlobalState} from 'types/store';

import SystemAnalytics from './system_analytics';

function mapStateToProps(state: GlobalState) {
    return {
        stats: state.entities.admin.analytics,
        config: getConfig(state),
        pluginStatHandlers: state.plugins.siteStatsHandlers,
    };
}

export default connect(mapStateToProps)(SystemAnalytics);
