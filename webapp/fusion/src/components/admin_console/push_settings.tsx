// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import type {WrappedComponentProps} from 'react-intl';
import {FormattedMessage, defineMessage, defineMessages, injectIntl} from 'react-intl';

import type {AdminConfig, EmailSettings} from '@mattermost/types/config';

import ExternalLink from 'components/external_link';

import {Constants, DocLinks} from 'utils/constants';

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
const PUSH_NOTIFICATIONS_MTPNS = 'mtpns';
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

export class PushSettings extends OLDAdminSettings<Props, State> {
    handleDropdownChange = (id: string, value: string) => {
        if (id === DROPDOWN_ID_SERVER_TYPE) {
            if (value === PUSH_NOTIFICATIONS_MTPNS) {
                this.setState({
                    pushNotificationServer: Constants.MTPNS,
                });
            } else if (value === PUSH_NOTIFICATIONS_CUSTOM &&
                this.state.pushNotificationServerType === PUSH_NOTIFICATIONS_MTPNS) {
                this.setState({
                    pushNotificationServer: '',
                });
            }
        }

        this.handleChange(id, value);
    };

    getConfigFromState = (config: Props['config']) => {
        config.EmailSettings.SendPushNotifications = this.state.pushNotificationServerType !== PUSH_NOTIFICATIONS_OFF;
        config.EmailSettings.PushNotificationServer = this.state.pushNotificationServer.trim();
        config.TeamSettings.MaxNotificationsPerChannel = this.state.maxNotificationsPerChannel;

        return config;
    };

    getStateFromConfig(config: Props['config']) {
        let pushNotificationServerType: EmailSettings['PushNotificationServerType'] = PUSH_NOTIFICATIONS_CUSTOM;
        if (!config.EmailSettings.SendPushNotifications) {
            pushNotificationServerType = PUSH_NOTIFICATIONS_OFF;
        } else if (config.EmailSettings.PushNotificationServer === Constants.MTPNS) {
            pushNotificationServerType = PUSH_NOTIFICATIONS_MTPNS;
        }

        let pushNotificationServer = config.EmailSettings.PushNotificationServer;
        if (pushNotificationServerType === PUSH_NOTIFICATIONS_MTPNS) {
            pushNotificationServer = Constants.MTPNS;
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
        pushNotificationServerTypes.push({value: PUSH_NOTIFICATIONS_MTPNS, text: this.props.intl.formatMessage({id: 'admin.email.mtpns', defaultMessage: 'Use TPNS connection to send notifications to iOS and Android apps'})});
        pushNotificationServerTypes.push({value: PUSH_NOTIFICATIONS_CUSTOM, text: this.props.intl.formatMessage({id: 'admin.email.selfPush', defaultMessage: 'Manually enter Push Notification Service location'})});

        let sendHelpText = null;
        let pushServerHelpText = null;
        if (this.state.pushNotificationServerType === PUSH_NOTIFICATIONS_OFF) {
            sendHelpText = (
                <FormattedMessage
                    id='admin.email.pushOffHelp'
                    defaultMessage='Please see <link>documentation on push notifications</link> to learn more about setup options.'
                    values={{
                        link: (msg) => (
                            <ExternalLink
                                href={DocLinks.SETUP_PUSH_NOTIFICATIONS}
                                location='push_settings'
                            >
                                {msg}
                            </ExternalLink>
                        ),
                    }}
                />
            );
        } else if (this.state.pushNotificationServerType === PUSH_NOTIFICATIONS_MTPNS) {
            pushServerHelpText = (
                <FormattedMessage
                    id='admin.email.mtpnsHelp'
                    defaultMessage='Download <linkIOS>Mattermost iOS app</linkIOS> from iTunes. Download <linkAndroid>Mattermost Android app</linkAndroid> from Google Play. Learn more about the <linkHPNS>Mattermost Hosted Push Notification Service</linkHPNS>.'
                    values={{
                        linkIOS: (msg) => (
                            <ExternalLink
                                href='https://mattermost.com/pl/ios-app/'
                                location='push_settings'
                            >
                                {msg}
                            </ExternalLink>
                        ),
                        linkAndroid: (msg) => (
                            <ExternalLink
                                href='https://mattermost.com/pl/android-app/'
                                location='push_settings'
                            >
                                {msg}
                            </ExternalLink>
                        ),
                        linkHPNS: (msg) => (
                            <ExternalLink
                                href={DocLinks.SETUP_PUSH_NOTIFICATIONS}
                                location='push_settings'
                            >
                                {msg}
                            </ExternalLink>
                        ),
                    }}
                />
            );
        } else {
            pushServerHelpText = (
                <FormattedMessage
                    id='admin.email.easHelp'
                    defaultMessage='Learn more about compiling and deploying your own mobile apps from an <link>Enterprise App Store</link>.'
                    values={{
                        link: (msg) => (
                            <ExternalLink
                                href='https://docs.mattermost.com/'
                                location='push_settings'
                            >
                                {msg}
                            </ExternalLink>
                        ),
                    }}
                />
            );
        }

        return (
            <SettingsGroup>
                <DropdownSetting
                    id={DROPDOWN_ID_SERVER_TYPE}
                    values={pushNotificationServerTypes}
                    label={<FormattedMessage {...messages.pushTitle}/>}
                    value={this.state.pushNotificationServerType}
                    onChange={this.handleDropdownChange}
                    helpText={sendHelpText}
                    setByEnv={this.isPushNotificationServerSetByEnv()}
                    disabled={this.props.isDisabled}
                />
                <TextSetting
                    id='pushNotificationServer'
                    label={<FormattedMessage {...messages.pushServerTitle}/>}
                    placeholder={defineMessage({id: 'admin.email.pushServerEx', defaultMessage: 'E.g.: "https://push-test.mattermost.com"'})}
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
