// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {useIntl} from 'react-intl';

import type {AdminConfig} from '@mattermost/types/config';

import {ldapTest} from 'actions/admin_actions';

import {ConsolePages, DocLinks} from 'utils/constants';

import {impactModifiers} from '../dashboard.data';
import {ItemStatus} from '../dashboard.type';
import type {ItemModel, Options} from '../dashboard.type';

const usesLDAP = async (
    config: Partial<AdminConfig>,
    formatMessage: ReturnType<typeof useIntl>['formatMessage'],
    options: Options,
): Promise<ItemModel> => {
    const testLdap = async (
        config: Partial<AdminConfig>,
    ): Promise<ItemStatus> => {
        let check = ItemStatus.INFO;

        if (!config.LdapSettings?.Enable) {
            return check;
        }

        const onSuccess = ({status}: any) => {
            if (status === 'OK') {
                check = ItemStatus.OK;
            }
        };

        await ldapTest(onSuccess);

        return check;
    };

    // something feels flawed in this check.
    const status = options.analytics?.TOTAL_USERS as number > 100 ? await testLdap(config) : ItemStatus.OK;

    return {
        id: 'ad-ldap',
        title: formatMessage({
            id: 'admin.reporting.workspace_optimization.ease_of_management.ldap.title',
            defaultMessage: 'AD/LDAP integration recommended',
        }),
        description: formatMessage({
            id: 'admin.reporting.workspace_optimization.ease_of_management.ldap.description',
            defaultMessage: 'You\'ve reached over 100 users! We recommend setting up AD/LDAP user authentication for easier onboarding as well as automated deactivations and role assignments.',
        }),
        configUrl: ConsolePages.AD_LDAP,
        configText: formatMessage({id: 'admin.reporting.workspace_optimization.ease_of_management.ldap.cta', defaultMessage: 'Try AD/LDAP'}),
        infoUrl: DocLinks.SETUP_LDAP,
        infoText: formatMessage({id: 'admin.reporting.workspace_optimization.cta.learnMore', defaultMessage: 'Learn more'}),
        telemetryAction: 'ad-ldap',
        status,
        scoreImpact: 22,
        impactModifier: impactModifiers[status],
    };
};

export const runEaseOfUseChecks = async (
    config: Partial<AdminConfig>,
    formatMessage: ReturnType<typeof useIntl>['formatMessage'],
    options: Options,
): Promise<ItemModel[]> => {
    const checks = [
        usesLDAP,
    ];

    const results = await Promise.all(checks.map((check) => check(config, formatMessage, options)));
    return results;
};
