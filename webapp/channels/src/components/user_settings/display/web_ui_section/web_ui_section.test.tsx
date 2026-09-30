// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {switchWebUI} from 'utils/web_ui';

import WebUISection from './web_ui_section';

jest.mock('utils/web_ui', () => ({
    ...jest.requireActual('utils/web_ui'),
    switchWebUI: jest.fn(),
}));

describe('components/user_settings/display/web_ui_section', () => {
    const stateWithSelection = (allowed: boolean) => ({
        entities: {
            general: {
                config: {
                    AllowUserWebUISelection: String(allowed),
                    DefaultWebUI: 'classic',
                },
            },
        },
    });

    const baseProps = {
        active: false,
        areAllSectionsInactive: true,
        updateSection: jest.fn(),
    };

    beforeEach(() => {
        jest.clearAllMocks();
    });

    test('is hidden when users cannot choose their web interface', () => {
        const {container} = renderWithContext(<WebUISection {...baseProps}/>, stateWithSelection(false));
        expect(container).toBeEmptyDOMElement();
    });

    test('shows the current web interface', () => {
        renderWithContext(<WebUISection {...baseProps}/>, stateWithSelection(true));
        expect(screen.getByText('Web interface')).toBeInTheDocument();
        expect(screen.getByText('Classic')).toBeInTheDocument();
    });

    test('switches the web interface on save', async () => {
        renderWithContext(
            <WebUISection
                {...baseProps}
                active={true}
                areAllSectionsInactive={false}
            />,
            stateWithSelection(true),
        );

        await userEvent.click(screen.getByLabelText('Fusion (preview)'));
        await userEvent.click(screen.getByText('Save'));

        expect(switchWebUI).toHaveBeenCalledWith('fusion');
    });

    test('does nothing when saving the current web interface', async () => {
        const updateSection = jest.fn();
        renderWithContext(
            <WebUISection
                {...baseProps}
                active={true}
                areAllSectionsInactive={false}
                updateSection={updateSection}
            />,
            stateWithSelection(true),
        );

        await userEvent.click(screen.getByText('Save'));

        expect(switchWebUI).not.toHaveBeenCalled();
        expect(updateSection).toHaveBeenCalledWith('');
    });
});
