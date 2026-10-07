// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {FormattedMessage, defineMessages} from 'react-intl';

import type {Team} from '@mattermost/types/teams';

import {Permissions} from 'mattermost-redux/constants';

import SystemPermissionGate from 'components/permissions_gates/system_permission_gate';
import TeamPermissionGate from 'components/permissions_gates/team_permission_gate';

import type {IconName} from 'fusion/components/icon';
import * as Utils from 'utils/utils';

import type {IntegrationsOptionRegistration} from 'types/store/plugins';

import IntegrationOption from './integration_option';

type Props = {
    siteName: string | undefined;
    enableIncomingWebhooks: boolean;
    enableOutgoingWebhooks: boolean;
    enableCommands: boolean;
    enableOAuthServiceProvider: boolean;
    enableOutgoingOAuthConnections: boolean;
    enableCustomEmoji: boolean;
    canCreateOrDeleteCustomEmoji: boolean;
    pluginOptions: IntegrationsOptionRegistration[];
    team: Team;
};

const messages = defineMessages({
    hooks: {id: 'integrations.section.hooks', defaultMessage: 'Webhooks and commands'},
    apps: {id: 'integrations.section.apps', defaultMessage: 'Apps and bots'},
    customization: {id: 'integrations.section.customization', defaultMessage: 'Customization'},
});

export default class Integrations extends React.PureComponent <Props> {
    componentDidMount() {
        this.updateTitle();
    }

    updateTitle = () => {
        const currentSiteName = this.props.siteName || '';
        document.title = Utils.localizeMessage({id: 'admin.sidebar.integrations', defaultMessage: 'Integrations'}) + ' - ' + this.props.team.display_name + ' ' + currentSiteName;
    };

    render() {
        // The overview groups the integrations as its side panel does, in rows of up to three tiles.
        const hooks = [];
        const apps = [];
        const customization = [];

        if (this.props.enableIncomingWebhooks) {
            hooks.push(
                <TeamPermissionGate
                    teamId={this.props.team.id}
                    permissions={[Permissions.MANAGE_INCOMING_WEBHOOKS, Permissions.MANAGE_OWN_INCOMING_WEBHOOKS]}
                    key='incomingWebhookPermission'
                >
                    <IntegrationOption
                        key='incomingWebhook'
                        icon='inbox'
                        title={
                            <FormattedMessage
                                id='integrations.incomingWebhook.title'
                                defaultMessage='Incoming Webhooks'
                            />
                        }
                        description={
                            <FormattedMessage
                                id='integrations.incomingWebhook.description'
                                defaultMessage='Incoming webhooks allow external integrations to send messages'
                            />
                        }
                        link={'/' + this.props.team.name + '/integrations/incoming_webhooks'}
                    />
                </TeamPermissionGate>,
            );
        }

        if (this.props.enableOutgoingWebhooks) {
            hooks.push(
                <TeamPermissionGate
                    teamId={this.props.team.id}
                    permissions={[Permissions.MANAGE_OUTGOING_WEBHOOKS, Permissions.MANAGE_OWN_OUTGOING_WEBHOOKS]}
                    key='outgoingWebhookPermission'
                >
                    <IntegrationOption
                        key='outgoingWebhook'
                        icon='export'
                        title={
                            <FormattedMessage
                                id='integrations.outgoingWebhook.title'
                                defaultMessage='Outgoing Webhooks'
                            />
                        }
                        description={
                            <FormattedMessage
                                id='integrations.outgoingWebhook.description'
                                defaultMessage='Outgoing webhooks allow external integrations to receive and respond to messages'
                            />
                        }
                        link={'/' + this.props.team.name + '/integrations/outgoing_webhooks'}
                    />
                </TeamPermissionGate>,
            );
        }

        if (this.props.enableCommands) {
            hooks.push(
                <TeamPermissionGate
                    teamId={this.props.team.id}
                    permissions={[Permissions.MANAGE_SLASH_COMMANDS, Permissions.MANAGE_OWN_SLASH_COMMANDS]}
                    key='commandPermission'
                >
                    <IntegrationOption
                        key='command'
                        icon='slash'
                        title={
                            <FormattedMessage
                                id='integrations.command.title'
                                defaultMessage='Slash Commands'
                            />
                        }
                        description={
                            <FormattedMessage
                                id='integrations.command.description'
                                defaultMessage='Slash commands send events to external integrations'
                            />
                        }
                        link={'/' + this.props.team.name + '/integrations/commands'}
                    />
                </TeamPermissionGate>,
            );
        }

        if (this.props.enableOAuthServiceProvider) {
            apps.push(
                <SystemPermissionGate
                    permissions={[Permissions.MANAGE_OAUTH]}
                    key='oauth2AppsPermission'
                >
                    <IntegrationOption
                        key='oauth2Apps'
                        icon='shield'
                        title={
                            <FormattedMessage
                                id='integrations.oauthApps.title'
                                defaultMessage='OAuth 2.0 Applications'
                            />
                        }
                        description={
                            <FormattedMessage
                                id='integrations.oauthApps.description'
                                defaultMessage='OAuth 2.0 allows external applications to make authorized requests to the Antimatter API'
                            />
                        }
                        link={'/' + this.props.team.name + '/integrations/oauth2-apps'}
                    />
                </SystemPermissionGate>,
            );
        }

        if (this.props.enableOutgoingOAuthConnections) {
            apps.push(
                <TeamPermissionGate
                    teamId={this.props.team.id}
                    permissions={[Permissions.MANAGE_OUTGOING_OAUTH_CONNECTIONS]}
                    key='outgoingOAuthConnectionsPermission'
                >
                    <IntegrationOption
                        key='outgoingOAuthConnections'
                        icon='link'
                        title={
                            <FormattedMessage
                                id='integrations.outgoingOAuthConnections.title'
                                defaultMessage='Outgoing OAuth Connections'
                            />
                        }
                        description={
                            <FormattedMessage
                                id='integrations.outgoingOAuthConnections.description'
                                defaultMessage='Outgoing OAuth Connections allow custom integrations to communicate to external systems'
                            />
                        }
                        link={'/' + this.props.team.name + '/integrations/outgoing-oauth2-connections'}
                    />
                </TeamPermissionGate>,
            );
        }

        apps.push(
            <SystemPermissionGate
                permissions={['manage_bots']}
                key='botsPermissions'
            >
                <IntegrationOption
                    icon='bot'
                    title={
                        <FormattedMessage
                            id='bots.manage.header'
                            defaultMessage='Bot Accounts'
                        />
                    }
                    description={
                        <FormattedMessage
                            id='bots.manage.description'
                            defaultMessage='Use bot accounts to integrate with Antimatter through plugins or the API'
                        />
                    }
                    link={'/' + this.props.team.name + '/integrations/bots'}
                />
            </SystemPermissionGate>,
        );

        if (this.props.enableCustomEmoji && this.props.canCreateOrDeleteCustomEmoji) {
            customization.push(
                <IntegrationOption
                    key='customEmoji'
                    icon='smile'
                    title={
                        <FormattedMessage
                            id='emoji_list.header'
                            defaultMessage='Custom Emoji'
                        />
                    }
                    description={
                        <FormattedMessage
                            id='integrations.customEmoji.description'
                            defaultMessage='Add emoji of your own, for everyone on the server to use'
                        />
                    }
                    link={'/' + this.props.team.name + '/emoji'}
                />,
            );
        }

        // Entries of plugins, such as the custom GIFs and stickers of Antimatter's GIFs plugin.
        this.props.pluginOptions.forEach((option) => {
            customization.push(
                <IntegrationOption
                    key={option.id}
                    icon={(option.fusionIcon || 'plug') as IconName}
                    title={<>{option.title}</>}
                    description={<>{option.description}</>}
                    onClick={option.action}
                />,
            );
        });

        const sections = [
            {key: 'hooks', title: <FormattedMessage {...messages.hooks}/>, options: hooks},
            {key: 'apps', title: <FormattedMessage {...messages.apps}/>, options: apps},
            {key: 'customization', title: <FormattedMessage {...messages.customization}/>, options: customization},
        ].filter((section) => section.options.length);

        return (
            <div className='backstage-content row'>
                <div className='backstage-header'>
                    <h1>
                        <FormattedMessage
                            id='integrations.header'
                            defaultMessage='Integrations'
                        />
                    </h1>
                </div>
                {sections.map((section) => (
                    <section
                        key={section.key}
                        className='integrations-section'
                    >
                        <h2>{section.title}</h2>
                        <div className='integrations-list'>
                            {section.options}
                        </div>
                    </section>
                ))}
            </div>
        );
    }
}
