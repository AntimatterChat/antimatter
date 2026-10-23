// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import React from 'react';

import type {CommunityProfile} from '@mattermost/types/community_profiles';

import {Client4} from 'mattermost-redux/client';

import {LAYER_ID} from 'fusion/components/layer';
import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import CommunityProfileDialog from './community_profile_dialog';

jest.mock('fusion/components/avatar', () => () => null);
const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));

function state(profile?: CommunityProfile, nameDisplay = 'nickname_full_name') {
    return {
        entities: {
            general: {config: {MaxFileSize: '1000', TeammateNameDisplay: 'username'}},
            users: {currentUserId: 'me', profiles: {me: TestHelper.getUserMock({id: 'me', username: 'maya.l', first_name: 'Maya', last_name: 'Lumen', nickname: 'Mayo'})}},
            teams: {
                currentTeamId: 't1',
                teams: {t1: TestHelper.getTeamMock({id: 't1', display_name: 'Lumen Labs', name_display: nameDisplay as never})},
                communityProfiles: {t1: profile ? {me: profile} : {}},
            },
            groups: {groups: {}},
            channels: {channels: {}},
        },
    } as never;
}

// One's profile in a community, from its menu: the nickname and picture shown there instead of the global ones.
describe('fusion/profiles/CommunityProfileDialog', () => {
    let patch: jest.SpyInstance;
    let upload: jest.SpyInstance;
    let reset: jest.SpyInstance;
    beforeEach(() => {
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);
        mockToast.mockClear();
        patch = jest.spyOn(Client4, 'patchCommunityProfile').mockImplementation(async (teamId, userId, change) => ({team_id: teamId, user_id: userId, nickname: change.nickname, update_at: 2}));
        upload = jest.spyOn(Client4, 'uploadCommunityAvatar').mockResolvedValue({team_id: 't1', user_id: 'me', avatar_update_at: 3, update_at: 3});
        reset = jest.spyOn(Client4, 'resetCommunityProfile').mockResolvedValue({team_id: 't1', user_id: 'me', update_at: 4});
    });

    afterEach(() => {
        document.getElementById(LAYER_ID)?.remove();
        patch.mockRestore();
        upload.mockRestore();
        reset.mockRestore();
    });

    test('is the community\'s, and previews how one shows there', async () => {
        renderWithContext(<CommunityProfileDialog onClose={jest.fn()}/>, state());
        expect(screen.getByRole('heading', {name: 'Your profile in Lumen Labs'})).toBeInTheDocument();
        expect(screen.getByText('Mayo')).toBeInTheDocument();
        await userEvent.type(screen.getByRole('textbox', {name: 'Nickname in this community'}), 'Captain');
        expect(screen.getByText('Captain')).toBeInTheDocument();
    });

    test('saves the nickname, trimmed, for the person only', async () => {
        renderWithContext(<CommunityProfileDialog onClose={jest.fn()}/>, state());
        const save = screen.getByRole('button', {name: 'Save nickname'});
        expect(save).toBeDisabled();
        await userEvent.type(screen.getByRole('textbox', {name: 'Nickname in this community'}), '  Captain ');
        await userEvent.click(save);
        expect(patch).toHaveBeenCalledWith('t1', 'me', {nickname: 'Captain'});
        await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Nickname saved for this community'));
    });

    test('a picture for the community, checked first; and back to the profile picture', async () => {
        renderWithContext(<CommunityProfileDialog onClose={jest.fn()}/>, state({team_id: 't1', user_id: 'me', avatar_update_at: 1, update_at: 1}));
        const input = screen.getByLabelText('Picture in this community', {selector: 'input'});
        await userEvent.upload(input, new File(['x'.repeat(2000)], 'big.png', {type: 'image/png'}));
        expect(screen.getByRole('alert')).toHaveTextContent('This image is too large.');
        expect(upload).not.toHaveBeenCalled();

        await userEvent.upload(input, new File(['x'], 'me.png', {type: 'image/png'}));
        expect(upload).toHaveBeenCalledWith('t1', 'me', expect.any(File));

        await userEvent.click(screen.getByRole('button', {name: 'Use my profile picture'}));
        expect(reset).toHaveBeenCalledWith('t1', 'me', {avatar: true});
    });

    test('says where the nickname shows when the community shows usernames', () => {
        renderWithContext(<CommunityProfileDialog onClose={jest.fn()}/>, state(undefined, 'username'));
        expect(screen.getByText('This community shows people by username: your nickname here only shows next to it in mention suggestions.')).toBeInTheDocument();
    });

    test('has no way to show or hide one\'s name: that\'s the community\'s privacy settings', () => {
        renderWithContext(<CommunityProfileDialog onClose={jest.fn()}/>, state());
        expect(screen.queryByRole('switch')).not.toBeInTheDocument();
    });
});
