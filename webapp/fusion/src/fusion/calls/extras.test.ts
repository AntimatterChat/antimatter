// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {VoiceParticipant} from './calls_api';
import {getScreenQuality, raisedHandOrder, setScreenQuality} from './extras';

const p = (sessionId: string, raisedHand: boolean, raisedHandAt = 0) => ({sessionId, raisedHand, raisedHandAt}) as unknown as VoiceParticipant;

describe('call extras', () => {
    test('raised hands are numbered in the order they went up', () => {
        expect(raisedHandOrder([p('a', true, 30), p('b', false), p('c', true, 10)])).toEqual({c: 1, a: 2});
    });

    test('the screen quality is remembered, and bad values fall back', () => {
        setScreenQuality({height: 1440, frameRate: 60});
        expect(getScreenQuality()).toEqual({height: 1440, frameRate: 60});
        localStorage.setItem('am-screen-quality', JSON.stringify({height: 123, frameRate: 7}));
        expect(getScreenQuality()).toEqual({height: 1080, frameRate: 30});
    });
});
