// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import {useSelector} from 'react-redux';

import {getConfig} from 'mattermost-redux/selectors/entities/general';

import type {GlobalState} from 'types/store';

// Antimatter: the built-in AI features run on the model connections an admin picked for them (AI › Built-in
// features), not through the AI chat app; the client config says which have one.

export function isAIRewriteAvailable(state: GlobalState): boolean {
    return getConfig(state).AIRewriteAvailable === 'true';
}

export function isAIRecapsAvailable(state: GlobalState): boolean {
    return getConfig(state).AIRecapsAvailable === 'true';
}

// useAIRewriteAvailable tells whether the composer may rewrite messages with AI.
export function useAIRewriteAvailable(): boolean {
    return useSelector(isAIRewriteAvailable);
}

// useAIRecapsAvailable tells whether recaps can be made.
export function useAIRecapsAvailable(): boolean {
    return useSelector(isAIRecapsAvailable);
}
