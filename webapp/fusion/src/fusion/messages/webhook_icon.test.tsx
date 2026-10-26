// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import Message from './message';

// A webhook's own picture (override_icon_url) goes through the image proxy when the server has one, as classic's
// post_profile_picture does: otherwise every reader's browser fetches it from the webhook's host, which learns their
// address, browser and when they read the message.
describe('fusion/messages: a webhook\'s picture', () => {
    const me = TestHelper.getUserMock({id: 'me', username: 'jason'});
    const creator = TestHelper.getUserMock({id: 'creator', username: 'marie'});
    const icon = 'https://tracker.example/p.png?c=42';

    const render = (config: Record<string, string>) => {
        const post = TestHelper.getPostMock({
            id: 'hook',
            type: '',
            channel_id: 'channel',
            user_id: creator.id,
            message: 'Build passed',
            props: {from_webhook: 'true', override_icon_url: icon},
        });
        return renderWithContext(
            <Message postId={post.id}/>,
            {
                entities: {
                    general: {config: {EnablePostIconOverride: 'true', ...config}},
                    users: {currentUserId: me.id, profiles: {me, creator}},
                    posts: {posts: {[post.id]: post}},
                    channels: {channels: {channel: TestHelper.getChannelMock({id: 'channel', team_id: 'team'})}},
                    teams: {currentTeamId: 'team', teams: {team: TestHelper.getTeamMock({id: 'team', name: 'team'})}},
                },
            } as never,
        );
    };

    const pictureSrc = (container: HTMLElement) => container.querySelector('.am-hook-av img')?.getAttribute('src');

    test('is fetched through the image proxy when there is one', () => {
        const {container} = render({HasImageProxy: 'true'});
        const src = pictureSrc(container);
        expect(src).toContain('/api/v4/image?url=');
        expect(src).toContain(encodeURIComponent(icon));
        expect(src?.startsWith(icon)).toBe(false);
    });

    test('is fetched as is without a proxy', () => {
        const {container} = render({HasImageProxy: 'false'});
        expect(pictureSrc(container)).toBe(icon);
    });
});
