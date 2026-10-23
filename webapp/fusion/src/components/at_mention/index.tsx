// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ConnectedProps} from 'react-redux';
import {connect} from 'react-redux';

import type {CommunityProfile} from '@mattermost/types/community_profiles';

import {getChannel} from 'mattermost-redux/selectors/entities/channels';
import {getAllGroupsForReferenceByName} from 'mattermost-redux/selectors/entities/groups';
import {getCurrentUserId, getUsersByUsername} from 'mattermost-redux/selectors/entities/users';

import {getCurrentScope, getNameDisplayIn, scopeOfChannel} from 'fusion/profiles/community_profiles';

import type {GlobalState} from 'types/store';

import {getMissingMentionedUsers} from './actions';
import AtMention from './at_mention';

const NO_PROFILES: Record<string, CommunityProfile> = {};

// Antimatter: a mention shows the person as the message's community shows them (its way of showing people, their
// nickname there); in direct and group messages, as the user chose.
function mapStateToProps(state: GlobalState, ownProps: {channelId?: string}) {
    const scope = scopeOfChannel(ownProps.channelId ? getChannel(state, ownProps.channelId) : undefined) ?? getCurrentScope(state);
    return {
        currentUserId: getCurrentUserId(state),
        teammateNameDisplay: getNameDisplayIn(state, scope),
        communityProfiles: (scope && state.entities.teams.communityProfiles?.[scope]) || NO_PROFILES,
        usersByUsername: getUsersByUsername(state),
        groupsByName: getAllGroupsForReferenceByName(state),
    };
}

const mapDispatchToProps = {
    getMissingMentionedUsers,
};

const connector = connect(mapStateToProps, mapDispatchToProps);
export type PropsFromRedux = ConnectedProps<typeof connector>;

export default connector(AtMention);
