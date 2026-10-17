// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import DirectRow from './direct_row';

jest.mock('fusion/shell/layout_context', () => ({useLayout: () => ({setNavOpen: jest.fn(), setHome: jest.fn()})}));
jest.mock('fusion/calls/markers', () => ({DirectCallMarker: () => null}));

const me = TestHelper.getUserMock({id: 'me', username: 'me'});
const alice = TestHelper.getUserMock({id: 'alice', username: 'alice'});
const dm = TestHelper.getChannelMock({id: 'dm', type: 'D', name: 'alice__me', display_name: 'alice', teammate_id: 'alice', team_id: ''});
const team = TestHelper.getTeamMock({id: 'team', name: 'antimatter'});

describe('fusion/sidebar/DirectRow', () => {
    const render = ({unread = 0, current = 'general', draft = ''}: {unread?: number; current?: string; draft?: string} = {}) => renderWithContext(
        <DirectRow
            channel={dm}
            dock={true}
        />,
        {
            entities: {
                users: {currentUserId: me.id, profiles: {me, alice}},
                teams: {currentTeamId: team.id, teams: {[team.id]: team}},
                channels: {
                    currentChannelId: current,
                    channels: {dm},
                    myMembers: {dm: TestHelper.getChannelMembershipMock({channel_id: 'dm', user_id: me.id, msg_count: 10 - unread, mention_count: unread, notify_props: {}})},
                    messageCounts: {dm: {total: 10, root: 10, urgent_mention_count: 0}},
                },
            },
            storage: {initialized: true, storage: draft ? {draft_dm: {timestamp: new Date(), value: {message: draft, fileInfos: [], uploadsInProgress: [], channelId: 'dm', rootId: '', createAt: 1, updateAt: 1}}} : {}},
        } as never,
    );

    test('rings the picture of a conversation with unread messages, and keeps its count quiet in place of the red badge', () => {
        const {container} = render({unread: 3});

        expect(container.querySelector('.am-av')).toHaveClass('am-unread-ring');
        expect(container.querySelector('.am-dm-count')).toHaveTextContent('3');
        expect(container.querySelector('.am-badge')).not.toBeInTheDocument();
        expect(screen.getByRole('link')).toHaveClass('am-unread');
    });

    test('shows neither ring nor count when everything is read', () => {
        const {container} = render();

        expect(container.querySelector('.am-av')).not.toHaveClass('am-unread-ring');
        expect(container.querySelector('.am-dm-count')).not.toBeInTheDocument();
    });

    test('marks a conversation with a draft waiting, open or not', () => {
        render({draft: 'see you at'});
        expect(screen.getByRole('img', {name: 'Draft waiting'})).toHaveAttribute('data-am-tip', 'Draft waiting');

        render({draft: 'see you at', current: 'dm'});
        expect(screen.getAllByRole('img', {name: 'Draft waiting'})).toHaveLength(2);
    });

    test('shows no pencil without a draft', () => {
        render();

        expect(screen.queryByRole('img', {name: 'Draft waiting'})).not.toBeInTheDocument();
    });

    test('names its close button in the Fusion tooltip, not the browser\'s', () => {
        render();

        const close = screen.getByRole('button', {name: 'Close conversation with alice'});
        expect(close).toHaveAttribute('data-am-tip', 'Close');
        expect(close).not.toHaveAttribute('title');
    });
});
