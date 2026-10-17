// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import Message from './message';

// A webhook posts under its creator's account: without the BOT tag, its posts could pass for that person's own
// (phishing). The System Console says every webhook post is tagged, whether or not it overrides the username.
describe('fusion/messages: the BOT tag of webhook posts', () => {
    const me = TestHelper.getUserMock({id: 'me', username: 'jason'});
    const creator = TestHelper.getUserMock({id: 'creator', username: 'marie', first_name: 'Marie', is_bot: false});

    const render = (props: Record<string, unknown>, config: Record<string, string> = {}) => {
        const post = TestHelper.getPostMock({id: 'hook', type: '', channel_id: 'channel', user_id: creator.id, message: 'Please confirm your password here', props});
        return renderWithContext(
            <Message postId={post.id}/>,
            {
                entities: {
                    general: {config: {EnablePostUsernameOverride: 'false', EnablePostIconOverride: 'false', ...config}},
                    users: {currentUserId: me.id, profiles: {me, creator}},
                    posts: {posts: {[post.id]: post}},
                    channels: {channels: {channel: TestHelper.getChannelMock({id: 'channel', team_id: 'team'})}},
                    teams: {currentTeamId: 'team', teams: {team: TestHelper.getTeamMock({id: 'team', name: 'team'})}},
                },
            } as never,
        );
    };

    test('a webhook post is tagged BOT when username overrides are off', () => {
        render({from_webhook: 'true'});
        expect(screen.getByText('BOT')).toBeInTheDocument();
    });

    test('a webhook post is tagged BOT when it doesn\'t override the username', () => {
        render({from_webhook: 'true'}, {EnablePostUsernameOverride: 'true'});
        expect(screen.getByText('BOT')).toBeInTheDocument();
    });

    test('a webhook post with an overridden username is tagged BOT, under that name', () => {
        render({from_webhook: 'true', override_username: 'Alerts'}, {EnablePostUsernameOverride: 'true'});
        expect(screen.getByText('BOT')).toBeInTheDocument();
        expect(screen.getByText('Alerts')).toBeInTheDocument();
    });

    test('a person\'s own post isn\'t tagged', () => {
        render({});
        expect(screen.queryByText('BOT')).not.toBeInTheDocument();
    });
});
