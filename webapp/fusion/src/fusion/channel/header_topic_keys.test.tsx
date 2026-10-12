// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import type {Channel} from '@mattermost/types/channels';

import {fireEvent, renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import HeaderTopic from './header_topic';

// The channel topic opens the whole header with Enter or Space, but Enter on a link in it follows the link.
describe('fusion/channel/HeaderTopic: keys', () => {
    const channel = TestHelper.getChannelMock({id: 'c1', team_id: 't1', type: 'O', display_name: 'Town', header: 'Notes on [the wiki](https://wiki.example.com)'}) as Channel;
    const render = () => renderWithContext(
        <HeaderTopic channel={channel}/>,
        {
            entities: {
                users: {currentUserId: 'me'},
                teams: {currentTeamId: 't1', teams: {t1: TestHelper.getTeamMock({id: 't1', name: 'lumen'})}},
                channels: {channels: {c1: channel}},
            },
        } as never,
    );

    beforeEach(() => {
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    test('Enter on the topic shows the whole header', () => {
        render();
        const topic = screen.getByRole('button', {name: /Notes on/});
        expect(topic).toHaveAttribute('aria-expanded', 'false');

        fireEvent.keyDown(topic, {key: 'Enter'});
        expect(topic).toHaveAttribute('aria-expanded', 'true');
    });

    test('Enter on a link in the topic is the link\'s: the header stays as it is', () => {
        render();
        const topic = screen.getByRole('button', {name: /Notes on/});

        fireEvent.keyDown(screen.getByRole('link', {name: 'the wiki'}), {key: 'Enter'});
        fireEvent.keyDown(screen.getByRole('link', {name: 'the wiki'}), {key: ' '});
        expect(topic).toHaveAttribute('aria-expanded', 'false');
    });
});
