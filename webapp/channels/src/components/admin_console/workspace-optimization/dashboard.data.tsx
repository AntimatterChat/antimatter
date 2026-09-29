// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {
    ChartLineIcon,
    ServerVariantIcon,
    ArrowUpBoldCircleOutlineIcon,
    TuneIcon,
    LockOutlineIcon,
    AccountMultipleOutlineIcon,
} from '@mattermost/compass-icons/components';
import type {AdminConfig} from '@mattermost/types/config';
import type {GlobalState} from '@mattermost/types/store';

import {getServerVersion} from 'mattermost-redux/selectors/entities/general';

import {ItemStatus} from './dashboard.type';
import type {DataModel, Options} from './dashboard.type';
import {runAccessChecks} from './dashboard_checks/access';
import {runConfigChecks} from './dashboard_checks/config';
import {runDataPrivacyChecks} from './dashboard_checks/data_privacy';
import {runEaseOfUseChecks} from './dashboard_checks/easy_management';
import {runPerformanceChecks} from './dashboard_checks/performance';
import {runUpdateChecks} from './dashboard_checks/updates';

export const impactModifiers: Record<ItemStatus, number> = {
    [ItemStatus.NONE]: 1,
    [ItemStatus.OK]: 1,
    [ItemStatus.INFO]: 0.5,
    [ItemStatus.WARNING]: 0.25,
    [ItemStatus.ERROR]: 0,
};

const getUpdatesData = async (
    config: Partial<AdminConfig>,
    formatMessage: ReturnType<typeof useIntl>['formatMessage'],
    options: Options,
) => ({
    title: formatMessage({
        id: 'admin.reporting.workspace_optimization.updates.title',
        defaultMessage: 'Server updates',
    }),
    description: formatMessage({
        id: 'admin.reporting.workspace_optimization.updates.description',
        defaultMessage: 'An update is available.',
    }),
    descriptionOk: formatMessage({
        id: 'admin.reporting.workspace_optimization.updates.descriptionOk',
        defaultMessage: 'Your workspace is completely up to date!',
    }),
    icon: (
        <div className='icon'>
            <ArrowUpBoldCircleOutlineIcon
                size={20}
                color={'var(--sys-center-channel-color)'}
            />
        </div>
    ),
    items: await runUpdateChecks(config, formatMessage, options),
});

const getConfigurationData = async (
    config: Partial<AdminConfig>,
    formatMessage: ReturnType<typeof useIntl>['formatMessage'],
    options: Options,
) => ({
    title: formatMessage({
        id: 'admin.reporting.workspace_optimization.configuration.title',
        defaultMessage: 'Configuration',
    }),
    description: formatMessage({
        id: 'admin.reporting.workspace_optimization.configuration.description',
        defaultMessage: 'You have configuration issues to resolve',
    }),
    descriptionOk: formatMessage({
        id: 'admin.reporting.workspace_optimization.configuration.descriptionOk',
        defaultMessage: 'You\'ve successfully configured SSL and Session Lengths!',
    }),
    icon: (
        <div className='icon'>
            <TuneIcon
                size={20}
                color={'var(--sys-center-channel-color)'}
            />
        </div>
    ),
    items: await runConfigChecks(config, formatMessage, options),
});

const getAccessData = async (
    config: Partial<AdminConfig>,
    formatMessage: ReturnType<typeof useIntl>['formatMessage'],
) => ({
    title: formatMessage({
        id: 'admin.reporting.workspace_optimization.access.title',
        defaultMessage: 'Workspace access',
    }),
    description: formatMessage({
        id: 'admin.reporting.workspace_optimization.access.description',
        defaultMessage: 'Web server configuration may be affecting access to your Antimatter workspace.',
    }),
    descriptionOk: formatMessage({
        id: 'admin.reporting.workspace_optimization.access.descriptionOk',
        defaultMessage: 'Your web server configuration is passing a live URL test!',
    }),
    icon: (
        <div className='icon'>
            <ServerVariantIcon
                size={20}
                color={'var(--sys-center-channel-color)'}
            />
        </div>
    ),
    items: await runAccessChecks(config, formatMessage),
});

const getPerformanceData = async (
    config: Partial<AdminConfig>,
    formatMessage: ReturnType<typeof useIntl>['formatMessage'],
    options: Options,
) => ({
    title: formatMessage({
        id: 'admin.reporting.workspace_optimization.performance.title',
        defaultMessage: 'Performance',
    }),
    description: formatMessage({
        id: 'admin.reporting.workspace_optimization.performance.description',
        defaultMessage: 'Your server would benefit from some performance tweaks.',
    }),
    descriptionOk: formatMessage({
        id: 'admin.reporting.workspace_optimization.performance.descriptionOk',
        defaultMessage: 'Your search performance suits your workspace usage!',
    }),
    icon: (
        <div className='icon'>
            <ChartLineIcon
                size={20}
                color={'var(--sys-center-channel-color)'}
            />
        </div>
    ),
    items: await runPerformanceChecks(config, formatMessage, options),
});

const getDataPrivacyData = async (
    config: Partial<AdminConfig>,
    formatMessage: ReturnType<typeof useIntl>['formatMessage'],
) => ({
    title: formatMessage({
        id: 'admin.reporting.workspace_optimization.data_privacy.title',
        defaultMessage: 'Data privacy',
    }),
    description: formatMessage({
        id: 'admin.reporting.workspace_optimization.data_privacy.description',
        defaultMessage: 'Get better insight and control over your data.',
    }),
    descriptionOk: formatMessage({
        id: 'admin.reporting.workspace_optimization.data_privacy.descriptionOk',
        defaultMessage: 'You\'ve enabled data retention and compliance features!',
    }),
    icon: (
        <div className='icon'>
            <LockOutlineIcon
                size={20}
                color={'var(--sys-center-channel-color)'}
            />
        </div>
    ),
    items: await runDataPrivacyChecks(config, formatMessage),
});

const getEaseOfManagementData = async (
    config: Partial<AdminConfig>,
    formatMessage: ReturnType<typeof useIntl>['formatMessage'],
    options: Options,
) => ({
    title: formatMessage({
        id: 'admin.reporting.workspace_optimization.ease_of_management.title',
        defaultMessage: 'Ease of management',
    }),
    description: formatMessage({
        id: 'admin.reporting.workspace_optimization.ease_of_management.description',
        defaultMessage: 'Make it easier to manage your Antimatter workspace.',
    }),
    descriptionOk: formatMessage({
        id: 'admin.reporting.workspace_optimization.ease_of_management.descriptionOk',
        defaultMessage: 'Your user authentication setup is appropriate based on your current usage!',
    }),
    icon: (
        <div className='icon'>
            <AccountMultipleOutlineIcon
                size={20}
                color={'var(--sys-center-channel-color)'}
            />
        </div>
    ),
    items: await runEaseOfUseChecks(config, formatMessage, options),
});

const useMetricsData = (
    config: Partial<AdminConfig>,
) => {
    const [loading, setLoading] = useState(true);
    const [data, setData] = useState<DataModel | undefined>(undefined);

    const {formatMessage} = useIntl();

    // get the currently installed server version
    const installedVersion = useSelector((state: GlobalState) => getServerVersion(state));
    const analytics = useSelector((state: GlobalState) => state.entities.admin.analytics) as unknown as Options['analytics'];

    const options: Options = useMemo(() => ({
        analytics,
        installedVersion,
    }), [analytics, installedVersion]);

    useEffect(() => {
        setLoading(true);
        const refreshData = async () => {
            const data = {
                updates: await getUpdatesData(config, formatMessage, options),
                configuration: await getConfigurationData(config, formatMessage, options),
                access: await getAccessData(config, formatMessage),
                performance: await getPerformanceData(config, formatMessage, options),
                dataPrivacy: await getDataPrivacyData(config, formatMessage),
                easyManagement: await getEaseOfManagementData(config, formatMessage, options),
            };

            return data;
        };

        refreshData().then((data) => {
            setData(data);
            setLoading(false);
        });
    }, [config, formatMessage, options]);

    return {
        data,
        loading,
    };
};

export default useMetricsData;
