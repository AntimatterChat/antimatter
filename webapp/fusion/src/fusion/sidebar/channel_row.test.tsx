// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import ChannelRow from './channel_row';

jest.mock('fusion/shell/layout_context', () => ({useLayout: () => ({setNavOpen: jest.fn()})}));
jest.mock('fusion/calls/hooks', () => ({useIsVoiceChannel: () => false}));
jest.mock('fusion/calls/markers', () => ({CallLiveMarker: () => null}));
jest.mock('plugins/pluggable', () => ({__esModule: true, default: () => null}));
jest.mock('./channel_threads', () => ({__esModule: true, default: () => null}));

const team = TestHelper.getTeamMock({id: 'team', name: 'antimatter'});
const secret = TestHelper.getChannelMock({id: 'secret', name: 'secret', display_name: 'Secret', type: 'P', team_id: team.id});

describe('fusion/sidebar/ChannelRow', () => {
    const render = ({current = 'town-square', draft = '', muted = false}: {current?: string; draft?: string; muted?: boolean} = {}) => renderWithContext(
        <ChannelRow channelId={secret.id}/>,
        {
            entities: {
                users: {currentUserId: 'me'},
                teams: {currentTeamId: team.id, teams: {[team.id]: team}},
                channels: {
                    currentChannelId: current,
                    channels: {[secret.id]: secret},
                    myMembers: {[secret.id]: TestHelper.getChannelMembershipMock({channel_id: secret.id, user_id: 'me', msg_count: 4, mention_count: 0}, muted ? {mark_unread: 'mention'} : {})},
                    messageCounts: {[secret.id]: {total: 4, root: 4, urgent_mention_count: 0}},
                },
            },
            storage: {initialized: true, storage: draft ? {draft_secret: {timestamp: new Date(), value: {message: draft, fileInfos: [], uploadsInProgress: [], channelId: secret.id, rootId: '', createAt: 1, updateAt: 1}}} : {}},
        } as never,
    );

    test('shows a pencil for the channel\'s glyph while a draft waits in it', () => {
        const {container} = render({draft: 'Half a thought'});

        expect(screen.getByRole('img', {name: 'Draft waiting'})).toHaveAttribute('data-am-tip', 'Draft waiting');
        expect(container.querySelector('.am-ch-ic use')).toHaveAttribute('href', '#am-i-pen');
    });

    test('keeps the channel\'s glyph without a draft, and in the channel you are writing in', () => {
        render();
        expect(screen.queryByRole('img', {name: 'Draft waiting'})).not.toBeInTheDocument();

        render({draft: 'Half a thought', current: secret.id});
        expect(screen.queryByRole('img', {name: 'Draft waiting'})).not.toBeInTheDocument();
    });

    test('a draft of spaces only is no draft', () => {
        render({draft: '   '});

        expect(screen.queryByRole('img', {name: 'Draft waiting'})).not.toBeInTheDocument();
    });

    test('names its marks and buttons in Fusion tooltips, without the browser\'s', () => {
        const {container} = render({muted: true});

        expect(screen.getByRole('img', {name: 'Private'})).toHaveAttribute('data-am-tip', 'Private');
        expect(screen.getByRole('img', {name: 'Muted until you turn it back on'})).toHaveAttribute('data-am-tip', 'Muted until you turn it back on');
        expect(screen.getByRole('button', {name: 'Channel options for Secret'})).toHaveAttribute('data-am-tip', 'Channel options');
        expect(container.querySelector('[title]')).not.toBeInTheDocument();
    });
});
