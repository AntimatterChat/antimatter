// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import {useEffect} from 'react';
import {useDispatch, useSelector} from 'react-redux';

import {getRecaps, getScheduledRecaps} from 'mattermost-redux/actions/recaps';
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

// hasAnyRecaps tells whether the person's recaps or scheduled recaps that were loaded include any.
export function hasAnyRecaps(state: GlobalState): boolean {
    const recaps = state.entities.recaps;
    return Boolean(recaps?.allIds?.length) || Object.keys(recaps?.scheduledRecaps || {}).length > 0;
}

// Whether the person's recaps were looked for, once per page load, to know if the entry shows without a connection.
let recapsLookedFor = false;

export function resetRecapsLookedFor() {
    recapsLookedFor = false;
}

// useRecapsEntryShown tells whether the Recaps entry shows, when the feature is on (enabled): with a model connection
// for recaps, or, without one, while the person has recaps to read or schedules to remove (they can't make new ones).
export function useRecapsEntryShown(enabled: boolean): boolean {
    const dispatch = useDispatch();
    const available = useAIRecapsAvailable();
    const hasRecaps = useSelector(hasAnyRecaps);
    useEffect(() => {
        if (enabled && !available && !hasRecaps && !recapsLookedFor) {
            recapsLookedFor = true;
            dispatch(getRecaps(0, 1));
            dispatch(getScheduledRecaps(0, 1));
        }
    }, [dispatch, enabled, available, hasRecaps]);
    return enabled && (available || hasRecaps);
}
