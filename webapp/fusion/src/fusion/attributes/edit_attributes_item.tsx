// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {Permissions} from 'mattermost-redux/constants';
import {getCustomProfileAttributes} from 'mattermost-redux/selectors/entities/general';
import {haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeamId} from 'mattermost-redux/selectors/entities/teams';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {MenuItem} from 'fusion/components/menu';
import {useUser} from 'fusion/hooks/users';
import {useToast} from 'fusion/shell/toast_context';
import {openDialog} from 'fusion/utils/modals';

import type {GlobalState} from 'types/store';

import {useCommunityAttributes} from './community_attributes';
import UserAttributesDialog, {shownServerAttributes, useCanEditServerAttributesOf} from './user_attributes_dialog';

export const USER_ATTRIBUTES_DIALOG = 'fusion_user_attributes';

// EditAttributesItem is "Edit attributes…" in a person's menu, for those who may set some of their attributes: the
// server's admins, the community's, and the person themselves when the community has attributes they set. A bot's
// menu has it for those who may set its server attributes: the server's admins and those who manage the bot.
export default function EditAttributesItem({userId, onDone}: {userId: string; onDone: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const user = useUser(userId);
    const me = useSelector(getCurrentUserId);
    const teamId = useSelector(getCurrentTeamId);
    const serverAdmin = useCanEditServerAttributesOf(user);
    const communityAdmin = useSelector((state: GlobalState) => haveITeamPermission(state, teamId, Permissions.MANAGE_TEAM));
    const serverFields = shownServerAttributes(useSelector(getCustomProfileAttributes));
    const communityFields = useCommunityAttributes(user?.is_bot ? undefined : teamId) || [];

    const server = serverAdmin && serverFields.length > 0;
    const community = communityFields.length > 0 && (communityAdmin || (userId === me && communityFields.some((f) => f.attrs?.managed !== 'admin')));
    if (!user || !(server || community)) {
        return null;
    }
    return (
        <MenuItem
            icon='tag'
            label={formatMessage({id: 'fusion.attributes.edit', defaultMessage: 'Edit attributes…'})}
            onClick={() => {
                onDone();
                dispatch(openDialog(USER_ATTRIBUTES_DIALOG, UserAttributesDialog, {userId, teamId, toast}));
            }}
        />
    );
}
