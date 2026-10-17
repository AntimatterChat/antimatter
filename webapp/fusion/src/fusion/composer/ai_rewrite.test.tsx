// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {Client4} from 'mattermost-redux/client';

import {RewriteAction} from 'components/advanced_text_editor/rewrite_action';

import {LAYER_ID} from 'fusion/components/layer';
import {renderHookWithContext, renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import AiRewriteMenu, {useAIRewriteAvailable} from './ai_rewrite';

// Antimatter: the composer's rewrite runs on the model connection an admin picked for it; the client config's
// AIRewriteAvailable says whether there is one. No agent is picked.
describe('fusion/composer/AiRewriteMenu', () => {
    beforeEach(() => {
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);
    });
    afterEach(() => {
        document.getElementById(LAYER_ID)?.remove();
    });

    const state = (config: Record<string, string> = {}) => ({
        entities: {
            general: {config},
            users: {currentUserId: 'me'},
            preferences: {myPreferences: {}},
        },
    } as never);

    test.each([
        ['true', true],
        ['false', false],
        [undefined, false],
    ])('rewrite is available with AIRewriteAvailable=%s: %s', (value, expected) => {
        const {result} = renderHookWithContext(() => useAIRewriteAvailable(), state(value === undefined ? {} : {AIRewriteAvailable: value}));
        expect(result.current).toBe(expected);
    });

    const render = (canRewrite: boolean, pluginItems: unknown[] = []) => {
        const props = {
            anchor: null,
            canRewrite,
            pluginItems: pluginItems as never,
            pluginProps: {} as never,
            message: 'hello there',
            rootId: '',
            onRewritten: jest.fn(),
            onError: jest.fn(),
            onBusy: jest.fn(),
            onClose: jest.fn(),
        };
        renderWithContext(<AiRewriteMenu {...props}/>, state({AIRewriteAvailable: String(canRewrite)}));
        return props;
    };

    test('rewrites without an agent', async () => {
        const rewrite = jest.spyOn(Client4, 'getAIRewrittenMessage').mockResolvedValue('Hello there.');
        const props = render(true);
        await userEvent.click(screen.getByRole('menuitem', {name: 'Make it shorter'}));
        await waitFor(() => expect(rewrite).toHaveBeenCalledWith('', 'hello there', RewriteAction.SHORTEN, undefined, ''));
        await waitFor(() => expect(props.onRewritten).toHaveBeenCalled());
        rewrite.mockRestore();
    });

    test('without a connection for the rewrite, only the apps\' items are offered', () => {
        render(false, [{id: 'p1', pluginId: 'mattermost-ai', text: 'Summarize', sortOrder: 1, icon: null, action: jest.fn()}]);
        expect(screen.queryByRole('menuitem', {name: 'Improve writing'})).not.toBeInTheDocument();
        expect(screen.queryByRole('menuitem', {name: 'Translate…'})).not.toBeInTheDocument();
        expect(screen.getByRole('menuitem', {name: 'Summarize'})).toBeInTheDocument();
    });
});
