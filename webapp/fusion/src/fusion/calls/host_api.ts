// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// The host controls that the Calls plugin's public API (window.antimatterCalls) leaves out, which its own UI and its
// /call commands run through its REST API: recording a call, making someone the host, and ending a call for
// everyone. The Calls server checks who may: the host records and makes someone else host (system admins too),
// and the call's moderators and whoever started it end it.

import {Client4} from 'mattermost-redux/client';

import type {GlobalState} from 'types/store';

const CALLS_PLUGIN_ID = 'com.mattermost.calls';
const CALLS_STATE = 'plugins-com.mattermost.calls';

// A request to the Calls server failed; status is its HTTP status (0 when it couldn't be sent).
export class CallsRequestError extends Error {
    status: number;

    constructor(message: string, status: number) {
        super(message);
        this.status = status;
    }
}

async function post(path: string, body?: object): Promise<void> {
    let response: Response;
    try {
        response = await fetch(`${Client4.getPluginRoute(CALLS_PLUGIN_ID)}${path}`, Client4.getOptions({
            method: 'post',
            body: body ? JSON.stringify(body) : undefined,
        }));
    } catch (e) {
        throw new CallsRequestError((e as Error)?.message || String(e), 0);
    }
    if (!response.ok) {
        let message = response.statusText;
        try {
            const data = await response.json();
            message = data?.detailed_error || data?.message || message;
        } catch {
            // Not JSON: keep the status text.
        }
        throw new CallsRequestError(message, response.status);
    }
}

// The routes take the call's channel (the Calls server has one call per channel).
export function startRecording(channelId: string): Promise<void> {
    return post(`/calls/${channelId}/recording/start`);
}

export function stopRecording(channelId: string): Promise<void> {
    return post(`/calls/${channelId}/recording/stop`);
}

export function makeHost(channelId: string, userId: string): Promise<void> {
    return post(`/calls/${channelId}/host/make`, {new_host_id: userId});
}

export function endCallForEveryone(channelId: string): Promise<void> {
    return post(`/calls/${channelId}/host/end`);
}

// isRecordingAvailable tells whether calls can be recorded on this server (Enable call recordings, with a job
// service to record them), from the Calls plugin's configuration.
export function isRecordingAvailable(state: GlobalState): boolean {
    const slice = (state as unknown as Record<string, {callsConfig?: {EnableRecordings?: boolean}} | undefined>)[CALLS_STATE];
    return Boolean(slice?.callsConfig?.EnableRecordings);
}
