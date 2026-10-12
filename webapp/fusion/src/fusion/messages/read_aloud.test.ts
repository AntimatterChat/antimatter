// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {act, renderHook} from '@testing-library/react';

import {readAloud, stopReading, useReadingPostId} from './read_aloud';

// A speech download the test answers when it wants: res() gives the server's response.
function pendingSpeech() {
    let answer: (blob: Blob) => void = () => {};
    const response = new Promise<Response>((resolve) => {
        answer = (blob: Blob) => resolve({ok: true, status: 200, blob: () => Promise.resolve(blob)} as unknown as Response);
    });
    return {response, res: answer};
}

class FakeAudio {
    static made: FakeAudio[] = [];
    static failNext = false;
    src: string;
    paused = true;
    constructor(src: string) {
        this.src = src;
        FakeAudio.made.push(this);
    }
    addEventListener() {}
    play() {
        if (FakeAudio.failNext) {
            FakeAudio.failNext = false;
            return Promise.reject(new Error('NotAllowedError'));
        }
        this.paused = false;
        return Promise.resolve();
    }
    pause() {
        this.paused = true;
    }
}

describe('fusion/messages/read_aloud', () => {
    const originalFetch = global.fetch;
    const originalAudio = window.Audio;
    let urls = 0;

    beforeEach(() => {
        FakeAudio.made = [];
        FakeAudio.failNext = false;
        window.Audio = FakeAudio as unknown as typeof Audio;
        URL.createObjectURL = jest.fn(() => `blob:${++urls}`);
        URL.revokeObjectURL = jest.fn();
    });

    afterEach(() => {
        stopReading();
        global.fetch = originalFetch;
        window.Audio = originalAudio;
    });

    test('a speech that arrives after a newer request is dropped, and only the last message is read', async () => {
        const first = pendingSpeech();
        const second = pendingSpeech();
        global.fetch = jest.fn().mockReturnValueOnce(first.response).mockReturnValueOnce(second.response);
        const {result} = renderHook(() => useReadingPostId());

        const readingFirst = readAloud('p1');
        const readingSecond = readAloud('p2');

        // The second answer comes first; the first one's, slower, comes after.
        await act(async () => {
            second.res(new Blob(['two']));
            await readingSecond;
        });
        await act(async () => {
            first.res(new Blob(['one']));
            await readingFirst;
        });

        expect(FakeAudio.made).toHaveLength(1);
        expect(FakeAudio.made[0].paused).toBe(false);
        expect(URL.createObjectURL).toHaveBeenCalledTimes(1);
        expect(result.current).toBe('p2');
    });

    test('a speech that arrives after Stop is dropped', async () => {
        const speech = pendingSpeech();
        global.fetch = jest.fn().mockReturnValueOnce(speech.response);

        const reading = readAloud('p1');
        stopReading();
        await act(async () => {
            speech.res(new Blob(['one']));
            await reading;
        });

        expect(FakeAudio.made).toHaveLength(0);
    });

    test('a failed playback leaves no message marked as being read, and frees its audio', async () => {
        const speech = pendingSpeech();
        global.fetch = jest.fn().mockReturnValueOnce(speech.response);
        FakeAudio.failNext = true;
        const {result} = renderHook(() => useReadingPostId());

        const reading = readAloud('p1');
        speech.res(new Blob(['one']));
        await act(async () => {
            await expect(reading).rejects.toThrow('NotAllowedError');
        });

        expect(result.current).toBe('');
        expect(URL.revokeObjectURL).toHaveBeenCalledWith(FakeAudio.made[0].src);
    });
});
