// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {CommunityAttributeField} from '@mattermost/types/properties_user';

import {TestHelper} from 'utils/test_helper';

// Shared fixtures of the attribute tests.

export const field = (id: string, attrs: Partial<CommunityAttributeField['attrs']> = {}, type = 'text'): CommunityAttributeField => ({
    id,
    group_id: 'g',
    name: id,
    type,
    attrs: {sort_order: 0, visibility: 'when_set', ...attrs},
    target_id: '',
    target_type: '',
    create_at: 0,
    update_at: 0,
    delete_at: 0,
    created_by: '',
    updated_by: '',
} as unknown as CommunityAttributeField);

// bots are the loaded bot accounts, by user ID (their owner_id says whose they are).
export function attributesState(teamId: string, serverFields: Record<string, unknown> = {}, users: Record<string, unknown> = {}, bots: Record<string, unknown> = {}) {
    return {
        entities: {
            general: {config: {}, customProfileAttributes: serverFields},
            bots: {accounts: bots},
            users: {
                currentUserId: 'me',
                profiles: {
                    me: TestHelper.getUserMock({id: 'me', username: 'me', first_name: '', last_name: '', nickname: ''}),
                    sofia: TestHelper.getUserMock({id: 'sofia', username: 'sofia', first_name: '', last_name: '', nickname: ''}),
                    bot: TestHelper.getUserMock({id: 'bot', username: 'bot', is_bot: true}),
                    ...users,
                },
            },
            teams: {currentTeamId: teamId, teams: {[teamId]: TestHelper.getTeamMock({id: teamId, display_name: 'Lumen'})}},
            channels: {channels: {}, myMembers: {}},
            preferences: {myPreferences: {}},
        },
    };
}
