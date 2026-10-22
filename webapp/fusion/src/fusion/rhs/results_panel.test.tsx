// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {RHSStates} from 'utils/constants';

import ResultsPanel from './results_panel';

// Antimatter: the Files tab (and its results) only show when ServiceSettings.EnableFileSearch (and
// FileSettings.EnableFileAttachments) are on, as the classic search results already hide.
describe('fusion/rhs/ResultsPanel: the Files tab', () => {
    const renderPanel = (config: Record<string, string>) => renderWithContext(
        <ResultsPanel rhsState={RHSStates.SEARCH}/>,
        {
            views: {
                rhs: {
                    rhsState: RHSStates.SEARCH,
                    searchType: 'messages',
                },
            },
            entities: {
                general: {config},
            },
        } as never,
    );

    test('shows the Files tab when file search is enabled', () => {
        renderPanel({EnableFileAttachments: 'true', EnableFileSearch: 'true'});
        expect(screen.getByText('Files')).toBeInTheDocument();
    });

    test('hides the Files tab when file search is disabled', () => {
        renderPanel({EnableFileAttachments: 'true', EnableFileSearch: 'false'});
        expect(screen.queryByText('Files')).not.toBeInTheDocument();
    });

    test('hides the Files tab when file attachments are disabled', () => {
        renderPanel({EnableFileAttachments: 'false', EnableFileSearch: 'true'});
        expect(screen.queryByText('Files')).not.toBeInTheDocument();
    });
});
