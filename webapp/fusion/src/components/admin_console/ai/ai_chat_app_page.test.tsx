// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';

import AIChatAppPage from './ai_chat_app_page';

jest.mock('components/admin_console/custom_plugin_settings', () => ({
    __esModule: true,
    default: ({match, isDisabled}: {match: {params: {plugin_id: string}}; isDisabled?: boolean}) => (
        <div data-testid='plugin-settings'>{`${match.params.plugin_id}|${String(Boolean(isDisabled))}`}</div>
    ),
}));

// AI › AI chat app is the AI chat app's own settings page, the one it had under Plugins.
describe('components/admin_console/ai/AIChatAppPage', () => {
    test('draws the AI chat app\'s settings page, whatever the address', () => {
        renderWithContext(
            <AIChatAppPage {...{match: {params: {}}, isDisabled: true} as never}/>,
        );
        expect(screen.getByTestId('plugin-settings')).toHaveTextContent('mattermost-ai|true');
    });
});
