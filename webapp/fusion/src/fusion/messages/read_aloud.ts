// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useSyncExternalStore} from 'react';

import {Client4} from 'mattermost-redux/client';

// Reading messages aloud (TextToSpeechSettings): the server turns the message into speech; one message is read at a
// time.

let reading: {postId: string; audio: HTMLAudioElement; url: string} | null = null;
const listeners = new Set<() => void>();
const notify = () => listeners.forEach((cb) => cb());

export function stopReading() {
    if (reading) {
        reading.audio.pause();
        URL.revokeObjectURL(reading.url);
        reading = null;
        notify();
    }
}

// readAloud reads a message aloud, stopping the one being read.
export async function readAloud(postId: string): Promise<void> {
    stopReading();
    const res = await fetch(`${Client4.getPostRoute(postId)}/speech`, Client4.getOptions({method: 'get'}));
    if (!res.ok) {
        throw new Error(`status ${res.status}`);
    }
    const url = URL.createObjectURL(await res.blob());
    const audio = new Audio(url);
    reading = {postId, audio, url};
    audio.addEventListener('ended', () => {
        if (reading?.audio === audio) {
            stopReading();
        }
    });
    notify();
    await audio.play();
}

// useReadingPostId is the message being read aloud, if any.
export function useReadingPostId(): string {
    return useSyncExternalStore((cb) => {
        listeners.add(cb);
        return () => listeners.delete(cb);
    }, () => reading?.postId || '');
}
