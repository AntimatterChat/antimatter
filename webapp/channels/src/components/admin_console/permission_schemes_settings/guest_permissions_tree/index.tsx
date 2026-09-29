// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import Permissions from 'mattermost-redux/constants/permissions';

import GuestPermissionsTree from './guest_permissions_tree';

export const GUEST_INCLUDED_PERMISSIONS = [
    Permissions.CREATE_PRIVATE_CHANNEL,
    Permissions.EDIT_POST,
    Permissions.DELETE_POST,
    Permissions.ADD_REACTION,
    Permissions.REMOVE_REACTION,
    Permissions.READ_CHANNEL,
    Permissions.UPLOAD_FILE,
    Permissions.EDIT_FILE_ATTACHMENT,
    Permissions.USE_CHANNEL_MENTIONS,
    Permissions.USE_GROUP_MENTIONS,
    Permissions.CREATE_POST,
];

export default GuestPermissionsTree;
