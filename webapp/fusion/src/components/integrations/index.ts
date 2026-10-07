// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {connect} from 'react-redux';

import {Permissions} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {haveISystemPermission, haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getMyTeams} from 'mattermost-redux/selectors/entities/teams';

import {getIntegrationsOptions} from 'selectors/integrations_options';

import type {GlobalState} from 'types/store';

import Integrations from './integrations';

function mapStateToProps(state: GlobalState) {
    const config = getConfig(state);
    const siteName = config.SiteName;
    const enableIncomingWebhooks = config.EnableIncomingWebhooks === 'true';
    const enableOutgoingWebhooks = config.EnableOutgoingWebhooks === 'true';
    const enableCommands = config.EnableCommands === 'true';
    const enableOAuthServiceProvider = config.EnableOAuthServiceProvider === 'true';
    const enableOutgoingOAuthConnections = config.EnableOutgoingOAuthConnections === 'true';
    const enableCustomEmoji = config.EnableCustomEmoji === 'true';

    // As in the integrations side panel: custom emoji show for those who may add or delete some.
    const canCreateOrDeleteCustomEmoji = haveISystemPermission(state, {permission: Permissions.CREATE_EMOJIS}) ||
        haveISystemPermission(state, {permission: Permissions.DELETE_EMOJIS}) ||
        getMyTeams(state).some((t) => haveITeamPermission(state, t.id, Permissions.CREATE_EMOJIS) || haveITeamPermission(state, t.id, Permissions.DELETE_EMOJIS));

    return {
        siteName,
        enableIncomingWebhooks,
        enableOutgoingWebhooks,
        enableCommands,
        enableOAuthServiceProvider,
        enableOutgoingOAuthConnections,
        enableCustomEmoji,
        canCreateOrDeleteCustomEmoji,
        pluginOptions: getIntegrationsOptions(state),
    };
}

export default connect(mapStateToProps)(Integrations);
