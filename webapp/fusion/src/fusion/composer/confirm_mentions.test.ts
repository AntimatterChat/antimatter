// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {openDialog} from 'fusion/utils/modals';

import type {GlobalState} from 'types/store';

import {confirmChannelWideMentions} from './confirm_mentions';

jest.mock('fusion/utils/modals', () => ({openDialog: jest.fn(() => ({type: 'DIALOG'}))}));
jest.mock('mattermost-redux/actions/channels', () => ({
    getChannelStats: jest.fn(() => ({type: 'STATS'})),
    getChannelTimezones: jest.fn(() => ({type: 'TIMEZONES', data: ['Europe/Paris', 'America/New_York']})),
}));
jest.mock('mattermost-redux/selectors/entities/roles', () => ({haveIChannelPermission: () => true}));

describe('fusion/composer/confirmChannelWideMentions', () => {
    const state = (members: number, confirm = 'true') => ({
        entities: {
            channels: {channels: {ch: {id: 'ch', team_id: 'team'}}, stats: {ch: {channel_id: 'ch', member_count: members}}},
            general: {config: {EnableConfirmNotificationsToChannel: confirm}},
        },
    } as unknown as GlobalState);
    const run = (message: string, s: GlobalState) => {
        const dispatch = jest.fn((a) => a);
        return {dispatch, result: confirmChannelWideMentions('ch', message)(dispatch as never, () => s, undefined)};
    };

    beforeEach(() => jest.clearAllMocks());

    test('sends messages without @all, @channel or @here right away', async () => {
        expect((await run('hello', state(50)).result).data).toBe(true);
        expect(openDialog).not.toHaveBeenCalled();
    });

    test('sends right away in small channels, or when the server doesn\'t ask', async () => {
        expect((await run('@all hi', state(4)).result).data).toBe(true);
        expect((await run('@all hi', state(50, 'false')).result).data).toBe(true);
        expect(openDialog).not.toHaveBeenCalled();
    });

    test('asks first, with how many people and timezones it reaches', async () => {
        const {result} = run('@all lunch is here', state(12));
        await Promise.resolve();
        await Promise.resolve();

        const props = (openDialog as jest.Mock).mock.calls[0][2];
        expect(props).toMatchObject({mentions: ['@all'], memberNotifyCount: 11, channelTimezoneCount: 2});

        props.onConfirm();
        expect((await result).data).toBe(true);
    });

    test('doesn\'t send when the dialog is closed without confirming', async () => {
        const {result} = run('@here quick question', state(12));
        await Promise.resolve();
        await Promise.resolve();

        (openDialog as jest.Mock).mock.calls[0][2].onExited();
        expect((await result).data).toBe(false);
    });
});
