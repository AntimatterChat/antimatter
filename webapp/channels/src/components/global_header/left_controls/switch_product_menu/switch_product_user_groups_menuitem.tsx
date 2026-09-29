// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {AccountMultipleOutlineIcon} from '@mattermost/compass-icons/components';

import {isCustomGroupsEnabled} from 'mattermost-redux/selectors/entities/preferences';

import {openModal} from 'actions/views/modals';

import * as Menu from 'components/menu';
import UserGroupsModal from 'components/user_groups_modal';

import {ModalIdentifiers} from 'utils/constants';

export default function ProductSwitcherUserGroupsMenuItem() {
    const dispatch = useDispatch();

    const isCustomUserGroupsEnabled = useSelector(isCustomGroupsEnabled);

    const handleClick = () => {
        dispatch(openModal({
            modalId: ModalIdentifiers.USER_GROUPS,
            dialogType: UserGroupsModal,
            dialogProps: {
                backButtonAction: handleClick,
            },
        }));
    };

    if (!isCustomUserGroupsEnabled) {
        return null;
    }

    return (
        <Menu.Item
            leadingElement={<AccountMultipleOutlineIcon size={18}/>}
            labels={
                <FormattedMessage
                    id='globalHeader.productSwitcherMenu.userGroupsMenuItem.label'
                    defaultMessage='User Groups'
                />
            }
            onClick={handleClick}
        />
    );
}
