// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {GlobalState} from 'types/store';

import {CallsRequestError, endCallForEveryone, isRecordingAvailable, makeHost, startRecording, stopRecording} from './host_api';

describe('fusion/calls/host_api', () => {
    const realFetch = global.fetch;
    let fetchMock: jest.Mock;
    beforeEach(() => {
        fetchMock = jest.fn(() => Promise.resolve({ok: true, status: 200, json: () => Promise.resolve({})}));
        global.fetch = fetchMock as unknown as typeof fetch;
    });
    afterEach(() => {
        global.fetch = realFetch;
    });

    const call = (i = 0) => ({url: fetchMock.mock.calls[i][0] as string, options: fetchMock.mock.calls[i][1] as RequestInit});

    test('uses the Calls server\'s routes, as its /call commands do', async () => {
        await startRecording('ch');
        await stopRecording('ch');
        await makeHost('ch', 'u1');
        await endCallForEveryone('ch');

        expect(call(0).url).toMatch(/\/plugins\/com\.mattermost\.calls\/calls\/ch\/recording\/start$/);
        expect(call(1).url).toMatch(/\/plugins\/com\.mattermost\.calls\/calls\/ch\/recording\/stop$/);
        expect(call(2).url).toMatch(/\/plugins\/com\.mattermost\.calls\/calls\/ch\/host\/make$/);
        expect(JSON.parse(call(2).options.body as string)).toEqual({new_host_id: 'u1'});
        expect(call(3).url).toMatch(/\/plugins\/com\.mattermost\.calls\/calls\/ch\/host\/end$/);
        for (let i = 0; i < 4; i++) {
            expect(call(i).options.method).toBe('post');
        }
    });

    test('a refusal rejects with the status and the server\'s reason', async () => {
        fetchMock.mockResolvedValueOnce({ok: false, status: 400, statusText: 'Bad Request', json: () => Promise.resolve({detailed_error: 'no permissions', status_code: 400})});
        const error = await endCallForEveryone('ch').catch((e) => e);
        expect(error).toBeInstanceOf(CallsRequestError);
        expect(error.status).toBe(400);
        expect(error.message).toBe('no permissions');
    });

    test('recordings are available when the Calls configuration enables them', () => {
        const state = (enabled?: boolean) => ({'plugins-com.mattermost.calls': {callsConfig: {EnableRecordings: enabled}}} as unknown as GlobalState);
        expect(isRecordingAvailable(state(true))).toBe(true);
        expect(isRecordingAvailable(state(false))).toBe(false);
        expect(isRecordingAvailable({} as GlobalState)).toBe(false);
    });
});
