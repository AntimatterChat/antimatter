// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {NavLink} from 'react-router-dom';

import type {Team} from '@mattermost/types/teams';

import {Permissions} from 'mattermost-redux/constants';

import SystemPermissionGate from 'components/permissions_gates/system_permission_gate';
import TeamPermissionGate from 'components/permissions_gates/team_permission_gate';

import Icon from 'fusion/components/icon';
import type {IconName} from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';
import {getHistory} from 'utils/browser_history';

type Props = {
    team: Team;
    enableCustomEmoji: boolean;
    enableIncomingWebhooks: boolean;
    enableOutgoingWebhooks: boolean;
    enableCommands: boolean;
    enableOAuthServiceProvider: boolean;
    enableOutgoingOAuthConnections: boolean;
    canCreateOrDeleteCustomEmoji: boolean;
    canManageIntegrations: boolean;
};

function Item({to, icon, label, exact = false}: {to: string; icon: IconName; label: string; exact?: boolean}) {
    return (
        <NavLink
            to={to}
            exact={exact}
            className={am('bs-item')}
            activeClassName={am('on')}
        >
            <Icon
                name={icon}
                size='sm'
            />
            <span>{label}</span>
        </NavLink>
    );
}

// IntegrationsNav is the Fusion side panel of the integrations and custom emoji pages: the way back to the team, then
// what the person may manage, with the permissions and settings the classic backstage sidebar checks.
export default function IntegrationsNav(props: Props) {
    const {formatMessage} = useIntl();
    const {team} = props;
    const base = `/${team.name}/integrations`;
    const teamExists = team.delete_at === 0;

    return (
        <nav
            className={am('bs-nav')}
            aria-label={formatMessage({id: 'fusion.integrations.nav', defaultMessage: 'Integrations'})}
        >
            <button
                type='button'
                className={am('bs-back')}
                onClick={() => getHistory().push(teamExists ? `/${team.name}` : '/')}
            >
                <Icon
                    name='chev'
                    size='sm'
                />
                {teamExists ? formatMessage({id: 'fusion.integrations.back', defaultMessage: 'Back to {team}'}, {team: team.display_name}) : formatMessage({id: 'fusion.integrations.backHome', defaultMessage: 'Back'})}
            </button>
            {props.canManageIntegrations && (
                <>
                    <h5>{formatMessage({id: 'fusion.integrations.title', defaultMessage: 'Integrations'})}</h5>
                    <Item
                        to={base}
                        exact={true}
                        icon='plug'
                        label={formatMessage({id: 'fusion.integrations.overview', defaultMessage: 'Overview'})}
                    />
                    {props.enableIncomingWebhooks && (
                        <TeamPermissionGate
                            permissions={[Permissions.MANAGE_INCOMING_WEBHOOKS, Permissions.MANAGE_OWN_INCOMING_WEBHOOKS]}
                            teamId={team.id}
                        >
                            <Item
                                to={`${base}/incoming_webhooks`}
                                icon='inbox'
                                label={formatMessage({id: 'fusion.integrations.incoming', defaultMessage: 'Incoming webhooks'})}
                            />
                        </TeamPermissionGate>
                    )}
                    {props.enableOutgoingWebhooks && (
                        <TeamPermissionGate
                            permissions={[Permissions.MANAGE_OUTGOING_WEBHOOKS, Permissions.MANAGE_OWN_OUTGOING_WEBHOOKS]}
                            teamId={team.id}
                        >
                            <Item
                                to={`${base}/outgoing_webhooks`}
                                icon='export'
                                label={formatMessage({id: 'fusion.integrations.outgoing', defaultMessage: 'Outgoing webhooks'})}
                            />
                        </TeamPermissionGate>
                    )}
                    {props.enableCommands && (
                        <TeamPermissionGate
                            permissions={[Permissions.MANAGE_SLASH_COMMANDS, Permissions.MANAGE_OWN_SLASH_COMMANDS]}
                            teamId={team.id}
                        >
                            <Item
                                to={`${base}/commands`}
                                icon='slash'
                                label={formatMessage({id: 'fusion.integrations.commands', defaultMessage: 'Slash commands'})}
                            />
                        </TeamPermissionGate>
                    )}
                    {props.enableOAuthServiceProvider && (
                        <SystemPermissionGate permissions={[Permissions.MANAGE_OAUTH]}>
                            <Item
                                to={`${base}/oauth2-apps`}
                                icon='shield'
                                label={formatMessage({id: 'fusion.integrations.oauthApps', defaultMessage: 'OAuth 2.0 apps'})}
                            />
                        </SystemPermissionGate>
                    )}
                    {props.enableOutgoingOAuthConnections && (
                        <TeamPermissionGate
                            permissions={[Permissions.MANAGE_OUTGOING_OAUTH_CONNECTIONS]}
                            teamId={team.id}
                        >
                            <Item
                                to={`${base}/outgoing-oauth2-connections`}
                                icon='link'
                                label={formatMessage({id: 'fusion.integrations.oauthConnections', defaultMessage: 'Outgoing OAuth connections'})}
                            />
                        </TeamPermissionGate>
                    )}

                    {/* Bot accounts only need the permission, even when creating bots is turned off. */}
                    <SystemPermissionGate permissions={['manage_bots', 'manage_others_bots']}>
                        <Item
                            to={`${base}/bots`}
                            icon='bot'
                            label={formatMessage({id: 'fusion.integrations.bots', defaultMessage: 'Bot accounts'})}
                        />
                    </SystemPermissionGate>
                </>
            )}
            {props.enableCustomEmoji && props.canCreateOrDeleteCustomEmoji && (
                <>
                    <h5>{formatMessage({id: 'fusion.integrations.customization', defaultMessage: 'Customization'})}</h5>
                    <Item
                        to={`/${team.name}/emoji`}
                        icon='smile'
                        label={formatMessage({id: 'fusion.integrations.emoji', defaultMessage: 'Custom emoji'})}
                    />
                </>
            )}
        </nav>
    );
}
