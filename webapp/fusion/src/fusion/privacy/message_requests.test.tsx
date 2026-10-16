// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {deletePreferences, savePreferences} from 'mattermost-redux/actions/preferences';
import {Client4} from 'mattermost-redux/client';
import {setMessageRequestsEnabled} from 'mattermost-redux/utils/channel_utils';

import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';
import {getHistory} from 'utils/browser_history';
import {TestHelper} from 'utils/test_helper';

import {BlockedNote, RequestBar, RequestsSection} from './message_requests';

jest.mock('mattermost-redux/actions/preferences', () => ({
    savePreferences: jest.fn(() => ({type: 'MOCK_SAVE_PREFERENCES'})),
    deletePreferences: jest.fn(() => ({type: 'MOCK_DELETE_PREFERENCES'})),
}));
jest.mock('mattermost-redux/actions/users', () => ({
    ...jest.requireActual('mattermost-redux/actions/users'),
    getMissingProfilesByIds: jest.fn(() => ({type: 'MOCK_MISSING_PROFILES'})),
}));
jest.mock('fusion/sidebar/direct_row', () => ({channel}: {channel: {display_name: string}}) => <div>{channel.display_name}</div>);
jest.mock('fusion/sidebar/last_message', () => ({useLastMessage: () => ''}));

const mockDialogs = {confirm: jest.fn(), blockPrompt: jest.fn()};
jest.mock('fusion/shell/dialogs_context', () => ({useDialogs: () => mockDialogs}));
const mockToast = jest.fn();
jest.mock('fusion/shell/toast_context', () => ({useToast: () => mockToast}));

const me = 'me';
const sofiaDM = TestHelper.getChannelMock({id: 'dm-sofia', name: 'me__sofia', display_name: 'sofia', type: 'D', team_id: '', last_post_at: 5, delete_at: 0});
const member = (request: string, msgCount = 0) => TestHelper.getChannelMembershipMock({
    channel_id: 'dm-sofia',
    user_id: me,
    msg_count: msgCount,
    notify_props: {...TestHelper.getChannelMembershipMock({}).notify_props, dm_request: request} as never,
});

function state({level = '', request = 'pending', enabled = true, neverAsk = false, total = 1} = {}) {
    const preferences: Record<string, unknown> = {};
    if (level) {
        preferences['user_block--sofia'] = {user_id: me, category: 'user_block', name: 'sofia', value: level};
    }
    if (neverAsk) {
        preferences['block_prompt--sofia'] = {user_id: me, category: 'block_prompt', name: 'sofia', value: 'off'};
    }
    return {
        entities: {
            general: {config: {EnableUserBlocking: String(enabled)}},
            users: {
                currentUserId: me,
                profiles: {me: TestHelper.getUserMock({id: me, username: me}), sofia: TestHelper.getUserMock({id: 'sofia', username: 'sofia', first_name: '', last_name: '', nickname: ''})},
            },
            teams: {currentTeamId: 't1', teams: {t1: TestHelper.getTeamMock({id: 't1', name: 'lumen'})}},
            channels: {
                channels: {'dm-sofia': sofiaDM},
                messageCounts: {'dm-sofia': {total, root: total}},
                myMembers: {'dm-sofia': member(request)},
            },
            preferences: {myPreferences: preferences},
        },
    };
}

describe('fusion/privacy/message_requests', () => {
    beforeEach(() => {
        setMessageRequestsEnabled(true);
        jest.spyOn(Client4, 'updateMessageRequest').mockResolvedValue(member('accepted') as never);
    });
    afterAll(() => setMessageRequestsEnabled(false));

    describe('RequestBar', () => {
        const render = (s = state()) => renderWithContext(
            <RequestBar
                channel={sofiaDM}
                userId='sofia'
            />,
            s as never,
        );

        test('accepting a request lets you reply', async () => {
            render();
            expect(screen.getByText('Message request from sofia')).toBeInTheDocument();
            await userEvent.click(screen.getByRole('button', {name: 'Accept'}));
            expect(Client4.updateMessageRequest).toHaveBeenCalledWith('dm-sofia', 'accept');
            await waitFor(() => expect(mockToast).toHaveBeenCalledWith('You can now talk with sofia'));
        });

        test('accepting someone you ignore stops ignoring them', async () => {
            render(state({level: 'ignore'}));
            expect(screen.getByText(/You ignore sofia\. Accepting stops ignoring them/)).toBeInTheDocument();
            await userEvent.click(screen.getByRole('button', {name: 'Accept'}));
            expect(deletePreferences).toHaveBeenCalledWith(me, [expect.objectContaining({category: 'user_block', name: 'sofia'})]);
        });

        test('a failure is told, not taken for success', async () => {
            jest.mocked(Client4.updateMessageRequest).mockRejectedValueOnce({message: 'Nope'});
            render();
            await userEvent.click(screen.getByRole('button', {name: 'Accept'}));
            await waitFor(() => expect(mockToast).toHaveBeenCalledWith('Nope'));
            expect(mockToast).not.toHaveBeenCalledWith('You can now talk with sofia');
        });

        test('ignoring a request dismisses it and leaves the conversation', async () => {
            const push = jest.spyOn(getHistory(), 'push').mockImplementation(() => undefined);
            render();
            await userEvent.click(screen.getByRole('button', {name: 'Ignore'}));
            expect(Client4.updateMessageRequest).toHaveBeenCalledWith('dm-sofia', 'dismiss');
            await waitFor(() => expect(push).toHaveBeenCalledWith('/lumen'));
            expect(mockToast).toHaveBeenCalledWith('Request from sofia ignored');
        });

        test('blocking asks first, then blocks', async () => {
            render();
            await userEvent.click(screen.getByRole('button', {name: 'Block'}));
            expect(mockDialogs.confirm).toHaveBeenCalledWith(expect.objectContaining({title: 'Block sofia?', danger: true, confirmLabel: 'Block'}));
            expect(savePreferences).not.toHaveBeenCalled();

            await mockDialogs.confirm.mock.calls[0][0].onConfirm();
            expect(savePreferences).toHaveBeenCalledWith(me, [expect.objectContaining({category: 'user_block', name: 'sofia', value: 'block'})]);
            expect(mockToast).toHaveBeenCalledWith('sofia is blocked');
        });
    });

    describe('ignoring a request offers to block a persistent sender', () => {
        const ignore = async (s: ReturnType<typeof state>, dismissals: string) => {
            jest.mocked(Client4.updateMessageRequest).mockResolvedValue({...member('dismissed'), notify_props: {...member('dismissed').notify_props, dm_request_dismissals: dismissals}} as never);
            jest.spyOn(getHistory(), 'push').mockImplementation(() => undefined);
            renderWithContext(
                <RequestBar
                    channel={sofiaDM}
                    userId='sofia'
                />,
                s as never,
            );
            await userEvent.click(screen.getByRole('button', {name: 'Ignore'}));
            await waitFor(() => expect(mockToast).toHaveBeenCalled());
        };

        test('after the second time', async () => {
            await ignore(state(), '2');
            expect(mockDialogs.blockPrompt).toHaveBeenCalledWith('sofia');
        });

        test('not the first time', async () => {
            await ignore(state(), '1');
            expect(mockDialogs.blockPrompt).not.toHaveBeenCalled();
        });

        test('with three of their messages unopened', async () => {
            await ignore(state({total: 3}), '0');
            expect(mockDialogs.blockPrompt).toHaveBeenCalledWith('sofia');
        });

        test('never when you asked not to be asked about them', async () => {
            await ignore(state({neverAsk: true}), '5');
            expect(mockDialogs.blockPrompt).not.toHaveBeenCalled();
        });
    });

    describe('RequestsSection', () => {
        test('lists the pending requests, collapsible, each with its Ignore button', async () => {
            renderWithContext(<RequestsSection/>, state() as never);
            const header = screen.getByRole('button', {name: /Message requests/});
            expect(header).toHaveTextContent('1');
            expect(header).toHaveAttribute('aria-expanded', 'true');
            expect(screen.getByRole('button', {name: 'Ignore the request from sofia'})).toBeInTheDocument();

            await userEvent.click(header);
            expect(header).toHaveAttribute('aria-expanded', 'false');
            expect(screen.queryByRole('button', {name: 'Ignore the request from sofia'})).not.toBeInTheDocument();
        });

        test('shows nothing without requests', () => {
            renderWithContext(<RequestsSection/>, state({request: 'accepted'}) as never);
            expect(screen.queryByRole('button', {name: /Message requests/})).not.toBeInTheDocument();
        });

        test('shows nothing when the admin turned blocking and requests off', () => {
            renderWithContext(<RequestsSection/>, state({enabled: false}) as never);
            expect(screen.queryByRole('button', {name: /Message requests/})).not.toBeInTheDocument();
        });

        test('leaves out requests from people you block', () => {
            renderWithContext(<RequestsSection/>, state({level: 'block'}) as never);
            expect(screen.queryByRole('button', {name: /Message requests/})).not.toBeInTheDocument();
        });
    });

    describe('BlockedNote', () => {
        test('says whom you block and unblocks them', async () => {
            renderWithContext(<BlockedNote userId='sofia'/>, state({level: 'block'}) as never);
            expect(screen.getByText('You blocked sofia')).toBeInTheDocument();
            await userEvent.click(screen.getByRole('button', {name: 'Unblock'}));
            expect(deletePreferences).toHaveBeenCalledWith(me, [expect.objectContaining({category: 'user_block', name: 'sofia'})]);
            expect(mockDialogs.confirm).not.toHaveBeenCalled();
            await waitFor(() => expect(mockToast).toHaveBeenCalledWith('sofia is unblocked'));
        });
    });
});
