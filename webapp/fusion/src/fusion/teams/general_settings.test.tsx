// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {patchTeam, removeTeamIcon, setTeamIcon} from 'mattermost-redux/actions/teams';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import GeneralSettings, {CommunityInfo} from './general_settings';

jest.mock('mattermost-redux/actions/channels', () => ({
    ...jest.requireActual('mattermost-redux/actions/channels'),
    getChannels: jest.fn(() => ({type: 'MOCK_GET_CHANNELS'})),
}));
jest.mock('mattermost-redux/actions/teams', () => ({
    ...jest.requireActual('mattermost-redux/actions/teams'),
    patchTeam: jest.fn(() => ({type: 'MOCK_PATCH_TEAM'})),
    setTeamIcon: jest.fn(() => ({type: 'MOCK_SET_ICON'})),
    removeTeamIcon: jest.fn(() => ({type: 'MOCK_REMOVE_ICON'})),
}));
const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));

function state({customGroups = true, icon = 0} = {}) {
    return {
        entities: {
            general: {config: {EnableCustomGroups: customGroups ? 'true' : 'false', MaxFileSize: '1000'}},
            teams: {currentTeamId: 't1', teams: {t1: TestHelper.getTeamMock({id: 't1', name: 'lumen', display_name: 'Lumen Labs', description: 'Optics', last_team_icon_update: icon})}},
            channels: {channels: {}, channelsInTeam: {}},
        },
    } as never;
}

describe('fusion/teams/GeneralSettings', () => {
    beforeEach(() => {
        jest.clearAllMocks();
    });

    test('has the community, what members do with its channels, and its layout', () => {
        renderWithContext(<GeneralSettings/>, state());
        expect(screen.getByRole('textbox', {name: 'Name'})).toHaveValue('Lumen Labs');
        expect(screen.getByRole('heading', {name: 'Channels'})).toBeInTheDocument();
        expect(screen.getByRole('group', {name: 'Kind of community'})).toBeInTheDocument();
        expect(screen.getByRole('heading', {name: 'Community layout'})).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Export as a template'})).toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Close'})).not.toBeInTheDocument();
    });

    test('no layout without roles on the server', () => {
        renderWithContext(<GeneralSettings/>, state({customGroups: false}));
        expect(screen.queryByRole('heading', {name: 'Community layout'})).not.toBeInTheDocument();
    });
});

describe('fusion/teams/CommunityInfo', () => {
    beforeEach(() => {
        jest.clearAllMocks();
    });

    test('saves the name and description together, and tells while they\'re unsaved', async () => {
        const setUnsaved = jest.fn();
        renderWithContext(<CommunityInfo setUnsaved={setUnsaved}/>, state());
        expect(screen.getByRole('button', {name: 'Save'})).toBeDisabled();

        const name = screen.getByRole('textbox', {name: 'Name'});
        await userEvent.clear(name);
        await userEvent.type(name, 'Lumen  ');
        await waitFor(() => expect(setUnsaved).toHaveBeenLastCalledWith(true));
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        expect(patchTeam).toHaveBeenCalledWith({id: 't1', display_name: 'Lumen', description: 'Optics'});
    });

    test('undoes the changes', async () => {
        const setUnsaved = jest.fn();
        renderWithContext(<CommunityInfo setUnsaved={setUnsaved}/>, state());
        await userEvent.type(screen.getByRole('textbox', {name: 'Description'}), ' and lasers');
        await userEvent.click(screen.getByRole('button', {name: 'Undo changes'}));
        expect(screen.getByRole('textbox', {name: 'Description'})).toHaveValue('Optics');
        await waitFor(() => expect(setUnsaved).toHaveBeenLastCalledWith(false));
    });

    test('a name too short isn\'t saved', async () => {
        renderWithContext(<CommunityInfo/>, state());
        const name = screen.getByRole('textbox', {name: 'Name'});
        await userEvent.clear(name);
        await userEvent.type(name, 'L');
        await userEvent.click(screen.getByRole('button', {name: 'Save'}));
        expect(screen.getByRole('alert')).toHaveTextContent('The name needs 2 to 64 characters.');
        expect(patchTeam).not.toHaveBeenCalled();
    });

    test('uploads an icon as soon as it\'s picked, and refuses other files', async () => {
        renderWithContext(<CommunityInfo/>, state());
        const input = screen.getByLabelText('Icon', {selector: 'input'});
        await userEvent.upload(input, new File(['x'], 'icon.png', {type: 'image/png'}));
        expect(setTeamIcon).toHaveBeenCalledWith('t1', expect.any(File));

        await userEvent.upload(input, new File(['x'.repeat(2000)], 'big.png', {type: 'image/png'}));
        expect(mockToast).toHaveBeenCalledWith('That image is too large.');
        expect(setTeamIcon).toHaveBeenCalledTimes(1);
    });

    test('removes the icon', async () => {
        renderWithContext(<CommunityInfo/>, state({icon: 12}));
        await userEvent.click(screen.getByRole('button', {name: 'Remove'}));
        expect(removeTeamIcon).toHaveBeenCalledWith('t1');
    });
});
