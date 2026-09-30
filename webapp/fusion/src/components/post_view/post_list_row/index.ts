// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {connect} from 'react-redux';
import {bindActionCreators} from 'redux';
import type {Dispatch} from 'redux';

import {getCurrentUserId} from 'mattermost-redux/selectors/entities/common';
import {getPost} from 'mattermost-redux/selectors/entities/posts';

import {emitShortcutReactToLastPostFrom} from 'actions/post_actions';
import {getShortcutReactToLastPostEmittedFrom} from 'selectors/emojis';

import type {GlobalState} from 'types/store';

import PostListRow from './post_list_row';
import type {PostListRowProps} from './post_list_row';

type OwnProps = Pick<PostListRowProps, 'listId'>;

function mapStateToProps(state: GlobalState, ownProps: OwnProps) {
    const shortcutReactToLastPostEmittedFrom = getShortcutReactToLastPostEmittedFrom(state);
    const post = getPost(state, ownProps.listId);
    const currentUserId = getCurrentUserId(state);
    const newMessagesSeparatorActions = state.plugins.components.NewMessagesSeparatorAction;

    const props: Pick<
        PostListRowProps,
    'shortcutReactToLastPostEmittedFrom' | 'post' | 'currentUserId' | 'newMessagesSeparatorActions'
    > = {
        shortcutReactToLastPostEmittedFrom,
        post,
        currentUserId,
        newMessagesSeparatorActions,
    };
    return props;
}

function mapDispatchToProps(dispatch: Dispatch) {
    return {
        actions: bindActionCreators({
            emitShortcutReactToLastPostFrom,
        }, dispatch),
    };
}

export default connect(mapStateToProps, mapDispatchToProps)(PostListRow);
