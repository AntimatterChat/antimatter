// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {connect} from 'react-redux';
import {bindActionCreators} from 'redux';
import type {Dispatch} from 'redux';

import type {Group} from '@mattermost/types/groups';

import {getGroups, getGroupsByUserIdPaginated, searchGroups} from 'mattermost-redux/actions/groups';
import {Permissions} from 'mattermost-redux/constants';
import {makeGetAllAssociatedGroupsForReference, makeGetMyAllowReferencedGroups, searchAllowReferencedGroups, searchMyAllowReferencedGroups, searchArchivedGroups, getArchivedGroups} from 'mattermost-redux/selectors/entities/groups';
import {haveISystemPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {openModal} from 'actions/views/modals';
import {setModalSearchTerm} from 'actions/views/search';
import {isModalOpen} from 'selectors/views/modals';

import {isTeam} from 'fusion/teams/teams';
import {ModalIdentifiers} from 'utils/constants';

import type {GlobalState} from 'types/store';

import UserGroupsModal from './user_groups_modal';

// Antimatter: of a community (communityId), the dialog lists and creates its teams. Without one, it's Mattermost's
// user groups.
type OwnProps = {
    communityId?: string;
};

// makeKeepCommunityTeams keeps a community's teams only, not its roles nor other communities' teams, giving back the
// same list while nothing changes.
const NO_GROUPS: Group[] = [];
function makeKeepCommunityTeams() {
    let lastGroups: Group[] = NO_GROUPS;
    let lastCommunityId = '';
    let lastResult: Group[] = NO_GROUPS;
    return (groups: Group[], communityId: string) => {
        if (groups !== lastGroups || communityId !== lastCommunityId) {
            lastGroups = groups;
            lastCommunityId = communityId;
            lastResult = groups.filter((g) => isTeam(g) && g.team_id === communityId);
        }
        return lastResult;
    };
}

function makeMapStateToProps() {
    const getAllAssociatedGroupsForReference = makeGetAllAssociatedGroupsForReference();
    const getMyAllowReferencedGroups = makeGetMyAllowReferencedGroups();
    const keepGroups = makeKeepCommunityTeams();
    const keepMyGroups = makeKeepCommunityTeams();
    const keepArchivedGroups = makeKeepCommunityTeams();

    return function mapStateToProps(state: GlobalState, ownProps: OwnProps) {
        const searchTerm = state.views.search.modalSearch;

        let groups: Group[] = [];
        let myGroups: Group[] = [];
        let archivedGroups: Group[] = [];
        if (searchTerm) {
            groups = searchAllowReferencedGroups(state, searchTerm, true);
            myGroups = searchMyAllowReferencedGroups(state, searchTerm, true);
            archivedGroups = searchArchivedGroups(state, searchTerm);
        } else {
            groups = getAllAssociatedGroupsForReference(state, true);
            myGroups = getMyAllowReferencedGroups(state, true);
            archivedGroups = getArchivedGroups(state);
        }

        const communityId = ownProps.communityId;
        if (communityId) {
            groups = keepGroups(groups, communityId);
            myGroups = keepMyGroups(myGroups, communityId);
            archivedGroups = keepArchivedGroups(archivedGroups, communityId);
        }

        return {
            showModal: isModalOpen(state, ModalIdentifiers.USER_GROUPS),
            groups,
            searchTerm,
            myGroups,
            archivedGroups,
            currentUserId: getCurrentUserId(state),
            canCreateCustomGroups: communityId ? haveITeamPermission(state, communityId, Permissions.MANAGE_TEAM_ROLES) : haveISystemPermission(state, {permission: Permissions.CREATE_CUSTOM_GROUP}),
        };
    };
}

function mapDispatchToProps(dispatch: Dispatch) {
    return {
        actions: bindActionCreators({
            getGroups,
            setModalSearchTerm,
            getGroupsByUserIdPaginated,
            searchGroups,
            openModal,
        }, dispatch),
    };
}

export default connect(makeMapStateToProps, mapDispatchToProps, null, {forwardRef: true})(UserGroupsModal);
