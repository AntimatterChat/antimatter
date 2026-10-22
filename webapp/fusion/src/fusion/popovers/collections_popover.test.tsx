// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import CollectionsPopover from './collections_popover';

// Antimatter: the Files tab only shows when ServiceSettings.EnableFileSearch (and
// FileSettings.EnableFileAttachments) are on, as the classic search results already hide.
describe('fusion/popovers/CollectionsPopover: the Files tab', () => {
    beforeEach(() => {
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    const channel = TestHelper.getChannelMock({id: 'channel1', name: 'test-channel'});

    const renderPopover = (config: Record<string, string>) => renderWithContext(
        <CollectionsPopover
            anchor={document.body}
            channel={channel}
            onClose={jest.fn()}
        />,
        {
            entities: {
                general: {config},
                channels: {channels: {[channel.id]: channel}},
            },
        } as never,
    );

    test('shows the Files tab when file search is enabled', () => {
        renderPopover({EnableFileAttachments: 'true', EnableFileSearch: 'true'});
        expect(screen.getByText('Files')).toBeInTheDocument();
    });

    test('hides the Files tab when file search is disabled', () => {
        renderPopover({EnableFileAttachments: 'true', EnableFileSearch: 'false'});
        expect(screen.queryByText('Files')).not.toBeInTheDocument();
    });

    test('hides the Files tab when file attachments are disabled', () => {
        renderPopover({EnableFileAttachments: 'false', EnableFileSearch: 'true'});
        expect(screen.queryByText('Files')).not.toBeInTheDocument();
    });

    test('falls back to Mentions when the Files tab was requested but file search is disabled', () => {
        renderWithContext(
            <CollectionsPopover
                anchor={document.body}
                channel={channel}
                initialTab='files'
                onClose={jest.fn()}
            />,
            {
                entities: {
                    general: {config: {EnableFileAttachments: 'true', EnableFileSearch: 'false'}},
                    channels: {channels: {[channel.id]: channel}},
                },
            } as never,
        );
        expect(screen.queryByText('Files')).not.toBeInTheDocument();
        expect(screen.getByRole('heading', {name: 'Mentions'})).toBeInTheDocument();
    });
});
