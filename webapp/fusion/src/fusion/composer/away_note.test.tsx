// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {renderWithContext, screen} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import AwayNote from './away_note';

describe('fusion/composer/AwayNote', () => {
    const me = TestHelper.getUserMock({id: 'me', username: 'me'});
    const bob = TestHelper.getUserMock({id: 'bob', username: 'bob', first_name: 'Bob'});
    const dm = TestHelper.getChannelMock({id: 'me__bob', name: 'bob__me', type: 'D', teammate_id: 'bob'} as never);
    const render = (status: string, dndEndTime = 0) => renderWithContext(
        <AwayNote channelId={dm.id}/>,
        {
            entities: {
                channels: {channels: {[dm.id]: dm}, myMembers: {}, channelsInTeam: {}},
                users: {currentUserId: 'me', profiles: {me, bob}, statuses: {bob: status}, dndEndTimes: {bob: dndEndTime}, profilesInChannel: {}},
            },
        } as never,
    );

    test('says who has Do Not Disturb on', () => {
        render('dnd');
        expect(screen.getByRole('note')).toHaveTextContent('Bob has Do Not Disturb on, so won\'t be notified.');
    });

    test('says until when', () => {
        render('dnd', Math.floor(Date.now() / 1000) + 3600);
        expect(screen.getByRole('note')).toHaveTextContent(/Bob has Do Not Disturb on until .+, so won't be notified\./);
    });

    test('says who is out of office', () => {
        render('ooo');
        expect(screen.getByRole('note')).toHaveTextContent('Bob is out of office.');
    });

    test('stays away for people who are around', () => {
        render('online');
        expect(screen.queryByRole('note')).not.toBeInTheDocument();
    });
});
