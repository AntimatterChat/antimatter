// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {connect} from 'react-redux';
import {withRouter} from 'react-router-dom';
import type {RouteComponentProps} from 'react-router-dom';
import {bindActionCreators} from 'redux';
import type {Dispatch} from 'redux';

import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {focusPost} from 'components/permalink_view/actions';

import type {GlobalState} from 'types/store';

import {onChannelByIdentifierEnter} from './actions';
import ChannelIdentifierRouter from './channel_identifier_router';
import type {Match} from './channel_identifier_router';

function mapStateToProps(state: GlobalState) {
    return {
        currentUserId: getCurrentUserId(state),
    };
}

function mapDispatchToProps(dispatch: Dispatch) {
    return {
        actions: bindActionCreators({
            onChannelByIdentifierEnter,
            focusPost,
        }, dispatch),
    };
}

// CenterChannel gives it where a permalink came from.
type OwnProps = RouteComponentProps<Match['params']> & {returnTo?: string};

const ConnectedChannelIdentifierRouter = connect(mapStateToProps, mapDispatchToProps)(ChannelIdentifierRouter);

export default withRouter<OwnProps, typeof ConnectedChannelIdentifierRouter>(ConnectedChannelIdentifierRouter);
