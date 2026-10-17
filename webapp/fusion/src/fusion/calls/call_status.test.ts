// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {callStatus} from './call_status';

describe('fusion/calls/call_status', () => {
    const base = {connecting: false, voice: false, direct: true, othersIn: false};

    test('a direct message\'s call is calling until someone else joins, then connected', () => {
        expect(callStatus({...base, connecting: true, callId: ''})).toBe('connecting');
        expect(callStatus({...base, callId: 'dm-call'})).toBe('calling');
        expect(callStatus({...base, callId: 'dm-call', othersIn: true})).toBe('call');
    });

    test('once answered, a group message\'s call stays connected when the others leave', () => {
        expect(callStatus({...base, callId: 'gm-call', othersIn: true})).toBe('call');
        expect(callStatus({...base, callId: 'gm-call', othersIn: false})).toBe('call');

        // Another call in the same conversation rings again.
        expect(callStatus({...base, callId: 'next-call'})).toBe('calling');
    });

    test('calls in channels are connected, voice channels say so, whoever is in them', () => {
        expect(callStatus({...base, direct: false, callId: 'channel-call'})).toBe('call');
        expect(callStatus({...base, direct: false, voice: true, callId: 'room'})).toBe('voice');
        expect(callStatus({...base, direct: false, voice: true, connecting: true, callId: 'room'})).toBe('connecting');
    });
});
