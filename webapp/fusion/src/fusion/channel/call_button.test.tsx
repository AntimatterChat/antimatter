// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {showCall} from 'fusion/calls/actions';
import {renderWithContext, screen, userEvent} from 'tests/react_testing_utils';
import {TestHelper} from 'utils/test_helper';

import CallButton, {OtherCallProviders} from './call_button';

const mockActions = {startOrJoinCall: jest.fn(), leaveCall: jest.fn()};
let mockAvailable = true;
let mockEnabled = true;
let mockVoice = false;
let mockCall: object | undefined;
let mockLocal: {channelId: string} | null = null;
let mockCallView: string | null = null;
jest.mock('fusion/calls/actions', () => ({useCallActions: () => mockActions, showCall: jest.fn()}));
jest.mock('fusion/calls/calls_api', () => ({getCallsAPI: () => ({selectors: {isCallsEnabled: () => mockEnabled}})}));
jest.mock('fusion/calls/hooks', () => ({
    useCallsAvailable: () => mockAvailable,
    useIsVoiceChannel: () => mockVoice,
    useCall: () => mockCall,
    useParticipants: () => [{}, {}, {}],
    useLocalCall: () => mockLocal,
}));
jest.mock('fusion/calls/ui_state', () => ({useCallsUI: () => ({callView: mockCallView})}));

const channel = TestHelper.getChannelMock({id: 'dm1', type: 'D', delete_at: 0});

describe('fusion/channel/CallButton', () => {
    beforeEach(() => {
        mockAvailable = true;
        mockEnabled = true;
        mockVoice = false;
        mockCall = undefined;
        mockLocal = null;
        mockCallView = null;
        jest.clearAllMocks();
    });

    test('starts a call (ringing the other person of a direct message)', async () => {
        renderWithContext(<CallButton channel={channel}/>);
        await userEvent.click(screen.getByRole('button', {name: 'Start a call'}));
        expect(mockActions.startOrJoinCall).toHaveBeenCalledWith('dm1');
    });

    test('joins the call in progress, saying how many are in it', async () => {
        mockCall = {};
        renderWithContext(<CallButton channel={channel}/>);
        const join = screen.getByRole('button', {name: 'Join the call, 3 people in it'});
        expect(join).toHaveTextContent('Join · 3');
        await userEvent.click(join);
        expect(mockActions.startOrJoinCall).toHaveBeenCalledWith('dm1');
    });

    // The voice connection panel at the bottom of the sidebar shows the call: the header doesn't, whether the call
    // is shown or not.
    test.each([null, 'dm1', 'other'])('in the call (call shown: %s): only leaving, no button to show the call', async (view) => {
        mockCall = {};
        mockLocal = {channelId: 'dm1'};
        mockCallView = view;
        renderWithContext(<CallButton channel={channel}/>);
        expect(screen.queryByRole('button', {name: 'Show the call'})).not.toBeInTheDocument();
        expect(screen.queryByRole('button', {name: 'Show the messages'})).not.toBeInTheDocument();
        expect(screen.getAllByRole('button')).toHaveLength(1);
        await userEvent.click(screen.getByRole('button', {name: 'Leave the call'}));
        expect(mockActions.leaveCall).toHaveBeenCalled();
        expect(showCall).not.toHaveBeenCalled();
    });

    test('no button where calls are off, in voice channels, or without the Calls app', () => {
        mockEnabled = false;
        const {rerender} = renderWithContext(<CallButton channel={channel}/>);
        expect(screen.queryByRole('button')).not.toBeInTheDocument();

        mockEnabled = true;
        mockVoice = true;
        rerender(<CallButton channel={channel}/>);
        expect(screen.queryByRole('button')).not.toBeInTheDocument();

        mockVoice = false;
        mockAvailable = false;
        rerender(<CallButton channel={channel}/>);
        expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });
});

describe('fusion/channel/OtherCallProviders', () => {
    const provider = (id: string, pluginId: string) => ({id, pluginId, button: `${id} call`, dropdownButton: `${id} meeting`, action: jest.fn()});
    const render = (providers: Array<ReturnType<typeof provider>>, ch = channel) => renderWithContext(
        <OtherCallProviders channel={ch}/>,
        {plugins: {components: {CallButton: providers}}, entities: {channels: {myMembers: {}}}} as never,
    );

    beforeEach(() => {
        if (!document.getElementById('am-layer')) {
            const layer = document.createElement('div');
            layer.id = 'am-layer';
            document.body.appendChild(layer);
        }
    });

    test('leaves Calls out and shows a single other provider as its button', async () => {
        const jitsi = provider('jitsi', 'jitsi');
        render([provider('calls', 'com.mattermost.calls'), jitsi]);
        expect(screen.queryByText('calls call')).not.toBeInTheDocument();
        await userEvent.click(screen.getByRole('button', {name: 'jitsi call'}));
        expect(jitsi.action).toHaveBeenCalled();
    });

    test('puts several providers in a menu', async () => {
        const zoom = provider('zoom', 'zoom');
        render([provider('jitsi', 'jitsi'), zoom]);
        await userEvent.click(screen.getByRole('button', {name: 'Other ways to call'}));
        await userEvent.click(screen.getByRole('menuitem', {name: 'zoom meeting'}));
        expect(zoom.action).toHaveBeenCalled();
    });

    test('none in archived channels', () => {
        render([provider('jitsi', 'jitsi')], {...channel, delete_at: 1});
        expect(screen.queryByRole('button')).not.toBeInTheDocument();
    });
});
