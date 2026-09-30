// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {connect} from 'react-redux';
import {bindActionCreators} from 'redux';
import type {Dispatch} from 'redux';

import {dismissError} from 'mattermost-redux/actions/errors';
import {Permissions} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {haveISystemPermission} from 'mattermost-redux/selectors/entities/roles';
import {getDisplayableErrors} from 'mattermost-redux/selectors/errors';

import type {GlobalState} from 'types/store';

import AnnouncementBarController from './announcement_bar_controller';

function mapStateToProps(state: GlobalState) {
    const canViewSystemErrors = haveISystemPermission(state, {permission: Permissions.MANAGE_SYSTEM});
    const config = getConfig(state);
    const errors = getDisplayableErrors(state);

    let latestError = null;
    if (errors && errors.length >= 1) {
        latestError = errors[0];
    }

    return {
        config,
        canViewSystemErrors,
        latestError,
    };
}

function mapDispatchToProps(dispatch: Dispatch) {
    const dismissFirstError = dismissError.bind(null, 0);
    return {
        actions: bindActionCreators({
            dismissError: dismissFirstError,
        }, dispatch),
    };
}

export default connect(mapStateToProps, mapDispatchToProps)(AnnouncementBarController);
