// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import type {WrappedComponentProps} from 'react-intl';
import {FormattedMessage, defineMessage, defineMessages, injectIntl} from 'react-intl';

import type {AdminConfig, EmailSettings} from '@mattermost/types/config';

import ExternalLink from 'components/external_link';

import {DocLinks} from 'utils/constants';

import DropdownSetting from './dropdown_setting';
import OLDAdminSettings from './old_admin_settings';
import type {BaseProps, BaseState} from './old_admin_settings';
import SettingsGroup from './settings_group';
import TextSetting from './text_setting';

type Props = BaseProps & {
    config: AdminConfig;
} & WrappedComponentProps;

type State = BaseState & {
    pushNotificationServer: string;
    pushNotificationServerType: EmailSettings['PushNotificationServerType'];
    maxNotificationsPerChannel: number;
};

const PUSH_NOTIFICATIONS_OFF = 'off';
const PUSH_NOTIFICATIONS_CUSTOM = 'custom';

const DROPDOWN_ID_SERVER_TYPE = 'pushNotificationServerType';

const messages = defineMessages({
    pushNotificationServer: {id: 'admin.environment.pushNotificationServer', defaultMessage: 'Push Notification Server'},
    pushTitle: {id: 'admin.email.pushTitle', defaultMessage: 'Enable Push Notifications: '},
    pushServerTitle: {id: 'admin.email.pushServerTitle', defaultMessage: 'Push Notification Server:'},
});

export const searchableStrings = [
    messages.pushNotificationServer,
    messages.pushTitle,
    messages.pushServerTitle,
];

// Push notification services run by Mattermost, Inc. (their test and hosted services) only deliver
// to Mattermost's own mobile apps, not to the Antimatter app.
export function isMattermostPushServer(url: string) {
    try {
        const host = new URL(url.trim()).hostname.toLowerCase();
        return host === 'mattermost.com' || host.endsWith('.mattermost.com');
    } catch {
        return false;
    }
}

export class PushSettings extends OLDAdminSettings<Props, State> {
    // Push notifications go through a push proxy the administrator runs: it needs an address.
    canSave = () => {
        return this.state.pushNotificationServerType === PUSH_NOTIFICATIONS_OFF ||
            this.state.pushNotificationServer.trim() !== '';
    };

    getConfigFromState = (config: Props['config']) => {
        config.EmailSettings.SendPushNotifications = this.state.pushNotificationServerType !== PUSH_NOTIFICATIONS_OFF;
        config.EmailSettings.PushNotificationServer = this.state.pushNotificationServer.trim();
        config.TeamSettings.MaxNotificationsPerChannel = this.state.maxNotificationsPerChannel;

        return config;
    };

    getStateFromConfig(config: Props['config']) {
        // Without a push notification server nothing is sent: that's the default, shown as off.
        // Any configured server is kept as it is, Mattermost's included.
        const pushNotificationServer = config.EmailSettings.PushNotificationServer ?? '';
        let pushNotificationServerType: EmailSettings['PushNotificationServerType'] = PUSH_NOTIFICATIONS_CUSTOM;
        if (!config.EmailSettings.SendPushNotifications || !pushNotificationServer.trim()) {
            pushNotificationServerType = PUSH_NOTIFICATIONS_OFF;
        }

        const maxNotificationsPerChannel = config.TeamSettings.MaxNotificationsPerChannel;

        return {
            pushNotificationServerType,
            pushNotificationServer,
            maxNotificationsPerChannel,
        };
    }

    isPushNotificationServerSetByEnv = () => {
        // Assume that if one of these has been set using an environment variable,
        // all of them have been set that way
        return this.isSetByEnv('EmailSettings.SendPushNotifications') ||
            this.isSetByEnv('EmailSettings.PushNotificationServer');
    };

    renderTitle() {
        return (<FormattedMessage {...messages.pushNotificationServer}/>);
    }

    renderSettings = () => {
        const pushNotificationServerTypes = [];
        pushNotificationServerTypes.push({value: PUSH_NOTIFICATIONS_OFF, text: this.props.intl.formatMessage({id: 'admin.email.pushOff', defaultMessage: 'Do not send push notifications'})});
        pushNotificationServerTypes.push({value: PUSH_NOTIFICATIONS_CUSTOM, text: this.props.intl.formatMessage({id: 'admin.email.ownPushProxy', defaultMessage: 'Send push notifications through your own push proxy'})});

        const setupLink = (msg: React.ReactNode) => (
            <ExternalLink
                href={DocLinks.SETUP_PUSH_NOTIFICATIONS}
                location='push_settings'
            >
                {msg}
            </ExternalLink>
        );

        let sendHelpText = null;
        let pushServerHelpText = null;
        if (this.state.pushNotificationServerType === PUSH_NOTIFICATIONS_OFF) {
            sendHelpText = (
                <FormattedMessage
                    id='admin.email.pushOffHelp'
                    defaultMessage='Please see <link>documentation on push notifications</link> to learn more about setup options.'
                    values={{link: setupLink}}
                />
            );
        } else {
            pushServerHelpText = (
                <>
                    <FormattedMessage
                        id='admin.email.ownPushProxyHelp'
                        defaultMessage='The address of your Antimatter push proxy, which delivers notifications to the Antimatter apps for iOS and Android. <link>Learn how to set it up</link>.'
                        values={{link: setupLink}}
                    />
                    {isMattermostPushServer(this.state.pushNotificationServer) && (
                        <>
                            {' '}
                            <FormattedMessage
                                id='admin.email.mattermostPushServer'
                                defaultMessage="This is a push notification service run by Mattermost, Inc.: it only delivers notifications to Mattermost's own mobile apps, not to the Antimatter app."
                            />
                        </>
                    )}
                </>
            );
        }

        return (
            <SettingsGroup>
                <DropdownSetting
                    id={DROPDOWN_ID_SERVER_TYPE}
                    values={pushNotificationServerTypes}
                    label={<FormattedMessage {...messages.pushTitle}/>}
                    value={this.state.pushNotificationServerType}
                    onChange={this.handleChange}
                    helpText={sendHelpText}
                    setByEnv={this.isPushNotificationServerSetByEnv()}
                    disabled={this.props.isDisabled}
                />
                <TextSetting
                    id='pushNotificationServer'
                    label={<FormattedMessage {...messages.pushServerTitle}/>}
                    placeholder={defineMessage({id: 'admin.email.pushServerExample', defaultMessage: 'E.g.: "https://push.example.com"'})}
                    helpText={pushServerHelpText}
                    value={this.state.pushNotificationServer}
                    onChange={this.handleChange}
                    disabled={this.props.isDisabled || this.state.pushNotificationServerType !== PUSH_NOTIFICATIONS_CUSTOM}
                    setByEnv={this.isSetByEnv('EmailSettings.PushNotificationServer')}
                />
                <TextSetting
                    id='maxNotificationsPerChannel'
                    type='number'
                    label={
                        <FormattedMessage
                            id='admin.team.maxNotificationsPerChannelTitle'
                            defaultMessage='Large Channel Notification Threshold:'
                        />
                    }
                    placeholder={defineMessage({id: 'admin.team.maxNotificationsPerChannelExample', defaultMessage: 'E.g.: "1000"'})}
                    helpText={
                        <FormattedMessage
                            id='admin.team.maxNotificationsPerChannelDescription'
                            defaultMessage='The maximum number of users that can be mentioned using @here, @all, or @channel. Above this limit, mentions and typing indicators are disabled, and users will see a system message if they try to use them.'
                        />
                    }
                    value={this.state.maxNotificationsPerChannel}
                    onChange={this.handleChange}
                    setByEnv={this.isSetByEnv('TeamSettings.MaxNotificationsPerChannel')}
                    disabled={this.props.isDisabled}
                />
            </SettingsGroup>
        );
    };
}

export default injectIntl(PushSettings);
