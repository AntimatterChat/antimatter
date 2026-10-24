// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {createMemoryHistory} from 'history';
import React from 'react';
import {Route} from 'react-router-dom';

import {onChannelByIdentifierEnter} from 'components/channel_layout/channel_identifier_router/actions';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import ForumTopicRoute from './topic_route';

jest.mock('components/channel_layout/channel_identifier_router/actions', () => ({onChannelByIdentifierEnter: jest.fn(() => ({type: 'MOCK_RESOLVE'}))}));
jest.mock('./topic_page', () => ({__esModule: true, default: ({rootId}: {rootId: string}) => <div>{`TopicPage:${rootId}`}</div>}));

describe('fusion/forum/ForumTopicRoute', () => {
    const channel = TestHelper.getChannelMock({id: 'c1', name: 'beam-questions', team_id: 'team'});

    const render = (channels: Record<string, unknown>) => {
        const history = createMemoryHistory({initialEntries: ['/lumen-labs/channels/beam-questions/topics/r0']});
        return renderWithContext(
            <Route path='/:team/channels/:identifier/topics/:rootId'>
                <ForumTopicRoute/>
            </Route>,
            {entities: {channels}} as never,
            {history},
        );
    };

    beforeEach(() => {
        jest.mocked(onChannelByIdentifierEnter).mockClear();
    });

    test('resolves the channel by its identifier, like the plain channel route', () => {
        render({currentChannelId: '', channels: {}});

        expect(onChannelByIdentifierEnter).toHaveBeenCalledWith(expect.objectContaining({
            match: expect.objectContaining({params: {team: 'lumen-labs', identifier: 'beam-questions', path: 'channels'}}),
        }));
    });

    test('shows a loading screen until the channel resolves', () => {
        render({currentChannelId: '', channels: {}});

        expect(screen.queryByText(/^TopicPage:/)).not.toBeInTheDocument();
    });

    test('once the channel resolves, shows the topic page', () => {
        render({currentChannelId: 'c1', channels: {c1: channel}});

        expect(screen.getByText('TopicPage:r0')).toBeInTheDocument();
    });
});
