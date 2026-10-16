// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {savePreferences} from 'mattermost-redux/actions/preferences';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import BlockPromptDialog from './block_prompt';

jest.mock('mattermost-redux/actions/preferences', () => ({
    savePreferences: jest.fn(() => ({type: 'MOCK_SAVE_PREFERENCES'})),
    deletePreferences: jest.fn(() => ({type: 'MOCK_DELETE_PREFERENCES'})),
}));
const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));

const me = 'me';
const state = {
    entities: {
        general: {config: {}},
        users: {
            currentUserId: me,
            profiles: {me: TestHelper.getUserMock({id: me}), sofia: TestHelper.getUserMock({id: 'sofia', username: 'sofia', first_name: '', last_name: '', nickname: ''})},
        },
        channels: {channels: {}, myMembers: {}},
        preferences: {myPreferences: {}},
    },
};

describe('fusion/privacy/BlockPromptDialog', () => {
    beforeEach(() => {
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const render = () => {
        const onClose = jest.fn();
        renderWithContext(
            <BlockPromptDialog
                userId='sofia'
                onClose={onClose}
            />,
            state as never,
        );
        return onClose;
    };

    test('blocks them', async () => {
        const onClose = render();
        expect(screen.getByRole('dialog', {name: 'Block sofia?'})).toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: 'Block'}));
        expect(savePreferences).toHaveBeenCalledWith(me, [{user_id: me, category: 'user_block', name: 'sofia', value: 'block'}]);
        await waitFor(() => expect(onClose).toHaveBeenCalled());
        expect(mockToast).toHaveBeenCalledWith('sofia is blocked');
    });

    test('"Not now" only closes it', async () => {
        const onClose = render();
        await userEvent.click(screen.getByRole('button', {name: 'Not now'}));
        expect(onClose).toHaveBeenCalled();
        expect(savePreferences).not.toHaveBeenCalled();
    });

    test('never asks again about them when told so', async () => {
        const onClose = render();
        await userEvent.click(screen.getByRole('button', {name: 'Don\'t ask again for this person'}));
        expect(savePreferences).toHaveBeenCalledWith(me, [{user_id: me, category: 'block_prompt', name: 'sofia', value: 'off'}]);
        expect(onClose).toHaveBeenCalled();
    });
});
