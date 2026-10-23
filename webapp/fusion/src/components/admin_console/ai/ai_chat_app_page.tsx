// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import store from 'stores/redux_store';

import CustomPluginSettings from 'components/admin_console/custom_plugin_settings';

import {AI_CHAT_APP_ID} from '../moved_console_routes';

type Props = React.ComponentProps<typeof CustomPluginSettings>;

// AIChatAppPage is the System Console's AI › AI chat app: the AI chat app's own settings page (its services,
// embeddings, web search, MCP...), the very page the app had under Plugins, with every AI setting now.
export default function AIChatAppPage(props: Props) {
    const match = {...props.match, params: {...props.match?.params, plugin_id: AI_CHAT_APP_ID}};
    return (
        <CustomPluginSettings
            {...props}
            match={match}
        />
    );
}

// isAIChatAppMissing tells whether the server's apps are known and the AI chat app isn't one of them (its page is
// then left out). While they aren't loaded yet the page stays, so that its address works when opened directly.
export function isAIChatAppMissing(): boolean {
    const plugins = store.getState().entities.admin.plugins;
    return Boolean(plugins && Object.keys(plugins).length > 0 && !plugins[AI_CHAT_APP_ID]);
}
