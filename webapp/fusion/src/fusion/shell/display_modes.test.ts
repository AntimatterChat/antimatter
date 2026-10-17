// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {renderHookWithContext} from 'tests/react_testing_utils';

import {useDisplayModes} from './display_modes';

describe('fusion/shell/useDisplayModes', () => {
    const pref = (category: string, name: string, value: string) => ({[`${category}--${name}`]: {user_id: 'me', category, name, value}});
    const render = (preferences: Record<string, unknown>) => renderHookWithContext(
        () => useDisplayModes(),
        {entities: {users: {currentUserId: 'me'}, preferences: {myPreferences: preferences}}} as never,
    );

    test('marks the page with the density and the simplified interface while the app is shown', () => {
        const {unmount} = render({...pref('fusion_display', 'spacious', 'true'), ...pref('fusion_display', 'simplified_interface', 'true')});

        expect(document.documentElement.dataset.amDensity).toBe('spacious');
        expect(document.documentElement.dataset.amSimplified).toBe('true');

        unmount();
        expect(document.documentElement.dataset.amDensity).toBeUndefined();
        expect(document.documentElement.dataset.amSimplified).toBeUndefined();
    });

    test('standard, and not simplified, by default', () => {
        const {unmount} = render({});

        expect(document.documentElement.dataset.amDensity).toBe('standard');
        expect(document.documentElement.dataset.amSimplified).toBeUndefined();
        unmount();
    });

    test('compact follows Mattermost\'s message display', () => {
        const {unmount} = render(pref('display_settings', 'message_display', 'compact'));

        expect(document.documentElement.dataset.amDensity).toBe('compact');
        unmount();
    });
});
