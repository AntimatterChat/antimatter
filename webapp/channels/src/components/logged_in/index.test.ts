// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import TestHelper from 'packages/mattermost-redux/test/test_helper';

import type {GlobalState} from 'types/store';

import {mapStateToProps} from './index';

describe('LoggedIn mapStateToProps', () => {
    const baseState = {
        entities: {
            channels: {
                currentChannelId: 'current-channel-id',
                myMembers: {},
                manuallyUnread: {},
            },
            general: {
                config: {},
                featureFlags: {},
            },
            users: {
                currentUserId: 'current-user-id',
                profiles: {
                    'current-user-id': TestHelper.fakeUserWithId('current-user-id'),
                },
            },
            preferences: {
                myPreferences: {},
            },
        },
    } as unknown as GlobalState;

    const baseProps = {
        match: {
            url: '/team/channel',
        },
    } as any;

    describe('other props', () => {
        it('should return correct props structure', () => {
            const props = mapStateToProps(baseState, baseProps);

            expect(props).toEqual({
                currentUser: expect.any(Object),
                currentChannelId: 'current-channel-id',
                isCurrentChannelManuallyUnread: false,
                mfaRequired: false,
                showTermsOfService: false,
            });
        });
    });
});
