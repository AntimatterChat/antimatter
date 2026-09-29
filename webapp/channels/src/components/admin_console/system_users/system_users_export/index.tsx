// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {Button} from '@mattermost/shared/components/button';
import {ReportDuration} from '@mattermost/types/reports';
import type {GlobalState} from '@mattermost/types/store';
import type {UserProfile} from '@mattermost/types/users';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {Preferences} from 'mattermost-redux/constants';
import {get} from 'mattermost-redux/selectors/entities/preferences';

import {startUsersBatchExport} from 'actions/views/admin';
import {openModal} from 'actions/views/modals';
import {getAdminConsoleUserManagementTableProperties} from 'selectors/views/admin';

import {ModalIdentifiers} from 'utils/constants';

import {ExportErrorModal} from './export_error_modal';
import {ExportUserDataModal} from './export_user_data_modal';

import {convertTableOptionsToUserReportOptions} from '../utils';

interface Props {
    currentUserId: UserProfile['id'];
    usersLenght: number;
}

export function SystemUsersExport(props: Props) {
    const dispatch = useDispatch();

    const skipDialog = useSelector((state: GlobalState) => get(state, Preferences.CATEGORY_REPORTING, Preferences.HIDE_BATCH_EXPORT_CONFIRM_MODAL, '')) === 'true';
    const tableFilterProps = useSelector(getAdminConsoleUserManagementTableProperties);
    const tableOptionsToUserReport = convertTableOptionsToUserReportOptions(tableFilterProps);
    if (tableOptionsToUserReport.date_range === undefined) {
        tableOptionsToUserReport.date_range = ReportDuration.AllTime;
    }

    async function doExport(checked?: boolean) {
        const {error} = await dispatch(startUsersBatchExport(tableOptionsToUserReport));
        if (error) {
            dispatch(openModal({
                modalId: ModalIdentifiers.EXPORT_ERROR_MODAL,
                dialogType: ExportErrorModal,
                dialogProps: {error},
            }));
            return;
        }

        if (checked) {
            dispatch(savePreferences(props.currentUserId, [{
                category: Preferences.CATEGORY_REPORTING,
                name: Preferences.HIDE_BATCH_EXPORT_CONFIRM_MODAL,
                user_id: props.currentUserId,
                value: 'true',
            }]));
        }
    }

    function handleExport() {
        if (!props.usersLenght) {
            return;
        }
        if (skipDialog) {
            doExport();
            return;
        }

        dispatch(openModal({
            modalId: ModalIdentifiers.EXPORT_USER_DATA_MODAL,
            dialogType: ExportUserDataModal,
            dialogProps: {onConfirm: doExport},
        }));
    }

    return (
        <Button
            onClick={handleExport}
            emphasis='tertiary'
            size='md'
            disabled={!props.usersLenght}
        >
            <span className='icon icon-download-outline'/>
            <FormattedMessage
                id='admin.system_users.exportButton'
                defaultMessage='Export'
            />
        </Button>
    );
}
