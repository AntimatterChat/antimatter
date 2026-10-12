// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {saveTheme} from 'mattermost-redux/actions/preferences';

import {act, fireEvent, renderWithContext, screen} from 'tests/react_testing_utils';

import CustomThemeEditor from './custom_theme_editor';

jest.mock('mattermost-redux/actions/preferences', () => ({
    ...jest.requireActual('mattermost-redux/actions/preferences'),
    saveTheme: jest.fn(() => ({type: 'MOCK_SAVE_THEME'})),
}));
jest.mock('utils/utils', () => ({
    ...jest.requireActual('utils/utils'),
    applyTheme: jest.fn(),
}));
jest.mock('fusion/shell/toast_context', () => ({useToast: () => jest.fn()}));

describe('fusion/modals/CustomThemeEditor', () => {
    beforeEach(() => {
        jest.useFakeTimers();
        (saveTheme as jest.Mock).mockClear();
    });

    afterEach(() => {
        jest.useRealTimers();
    });

    const pick = (colour: string) => {
        const accent = screen.getByLabelText('Accent');
        fireEvent.change(accent, {target: {value: colour}});
    };

    test('saves the colours once the picking stops', () => {
        const {unmount} = renderWithContext(<CustomThemeEditor/>);
        pick('#123456');
        expect(saveTheme).not.toHaveBeenCalled();

        act(() => {
            jest.advanceTimersByTime(600);
        });
        expect(saveTheme).toHaveBeenCalledTimes(1);

        // Nothing left to save when the editor closes.
        unmount();
        expect(saveTheme).toHaveBeenCalledTimes(1);
    });

    test('closing the editor while picking saves what was picked', () => {
        const {unmount} = renderWithContext(<CustomThemeEditor/>);
        pick('#123456');
        pick('#654321');
        unmount();

        expect(saveTheme).toHaveBeenCalledTimes(1);
        expect(saveTheme).toHaveBeenCalledWith('', expect.any(Object));

        // The save the timer had waiting doesn't come on top.
        act(() => {
            jest.advanceTimersByTime(600);
        });
        expect(saveTheme).toHaveBeenCalledTimes(1);
    });
});
