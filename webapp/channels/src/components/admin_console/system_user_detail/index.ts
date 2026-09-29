// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ConnectedProps} from 'react-redux';
import {connect} from 'react-redux';

import {getCustomProfileAttributeFields} from 'mattermost-redux/actions/general';
import {getUserPreferences} from 'mattermost-redux/actions/preferences';
import {addUserToTeam} from 'mattermost-redux/actions/teams';
import {updateUserActive, updateUserAuth, getUser, patchUser, updateUserMfa, getCustomProfileAttributeValues, saveCustomProfileAttribute, uploadProfileImage, setDefaultProfileImage} from 'mattermost-redux/actions/users';
import {getConfig, getCustomProfileAttributes} from 'mattermost-redux/selectors/entities/general';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {setNavigationBlocked} from 'actions/admin_actions';
import {openModal} from 'actions/views/modals';
import {getShowManageUserSettings} from 'selectors/admin_console';

import type {GlobalState} from 'types/store';

import SystemUserDetail from './system_user_detail';

function mapStateToProps(state: GlobalState) {
    const config = getConfig(state);
    const customProfileAttributeFields = getCustomProfileAttributes(state);

    const showManageUserSettings = getShowManageUserSettings(state);

    return {
        currentUserId: getCurrentUserId(state),
        mfaEnabled: config?.EnableMultifactorAuthentication === 'true' || false,
        maxFileSize: parseInt(config?.MaxFileSize || '', 10),
        ldapPictureAttributeSet: config?.LdapPictureAttributeSet === 'true',
        customProfileAttributeFields,
        showManageUserSettings,
    };
}

const mapDispatchToProps = {
    getUser,
    patchUser,
    updateUserAuth,
    updateUserActive,
    updateUserMfa,
    addUserToTeam,
    setNavigationBlocked,
    openModal,
    getUserPreferences,
    getCustomProfileAttributeFields,
    getCustomProfileAttributeValues,
    saveCustomProfileAttribute,
    uploadProfileImage,
    setDefaultProfileImage,
};
const connector = connect(mapStateToProps, mapDispatchToProps);

export type PropsFromRedux = ConnectedProps<typeof connector>;
export default connector(SystemUserDetail);
