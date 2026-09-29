// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import type {ReactNode} from 'react';
import {injectIntl} from 'react-intl';
import type {IntlShape} from 'react-intl';
import {Link} from 'react-router-dom';

import type {ClientConfig} from '@mattermost/types/config';

import ExternalLink from 'components/external_link';

import {AnnouncementBarTypes} from 'utils/constants';

import TextDismissableBar from '../text_dismissable_bar';

type Props = {
    config?: Partial<ClientConfig>;
    intl: IntlShape;
    canViewSystemErrors: boolean;
    siteURL: string;
};

const ConfigurationAnnouncementBar = (props: Props) => {
    const {formatMessage} = props.intl;

    if (props.config?.SendEmailNotifications !== 'true' &&
            props.config?.EnablePreviewModeBanner === 'true'
    ) {
        const emailMessage = formatMessage({
            id: 'announcement_bar.error.preview_mode',
            defaultMessage: 'Preview Mode: Email notifications have not been configured.',
        });

        return (
            <TextDismissableBar
                allowDismissal={true}
                text={emailMessage}
                type={AnnouncementBarTypes.SUCCESS}
            />
        );
    }

    if (props.canViewSystemErrors && props.config?.SiteURL === '') {
        const values = {
            linkSite: (msg: ReactNode[]) => (
                <ExternalLink
                    href={props.siteURL}
                    location='configuration_announcement_bar'
                >
                    {msg}
                </ExternalLink>
            ),
            linkConsole: (msg: ReactNode[]) => (
                <Link to='/admin_console/environment/web_server'>
                    {msg}
                </Link>
            ),
        };

        let siteURLMessage;
        if (props.config?.EnableSignUpWithGitLab === 'true') {
            siteURLMessage = formatMessage({
                id: 'announcement_bar.error.site_url_gitlab.full',
                defaultMessage: 'Please configure your <linkSite>site URL</linkSite> either on the <linkConsole>System Console</linkConsole> or, if you\'re using GitLab Antimatter, in gitlab.rb.',
            }, values);
        } else {
            siteURLMessage = formatMessage({
                id: 'announcement_bar.error.site_url.full',
                defaultMessage: 'Please configure your <linkSite>site URL</linkSite> on the <linkConsole>System Console</linkConsole>.',
            }, values);
        }

        return (
            <TextDismissableBar
                allowDismissal={true}
                text={siteURLMessage}
                type={AnnouncementBarTypes.ANNOUNCEMENT}
            />
        );
    }

    return null;
};

export default injectIntl(ConfigurationAnnouncementBar);
