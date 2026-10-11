// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useSyncExternalStore} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useSelector} from 'react-redux';

import Icon from 'fusion/components/icon';
import {showToast} from 'fusion/components/toast';
import {useDisplayName, useUser} from 'fusion/hooks/users';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {getCallsAPI} from './calls_api';
import type {CallsCaption, CallsTranscription} from './plugin_api/calls_public_api';

// Transcriptions of calls by a speech-to-text API (the Calls plugin's TranscribeFromClients): the host starts and
// stops them, everyone sees that the call is transcribed, and its live captions, which each user can hide.

const NO_TRANSCRIPTION: CallsTranscription = {running: false, startAt: 0, liveCaptions: false};
const NO_CAPTIONS: CallsCaption[] = [];

export function useTranscriptionAvailable(): boolean {
    return useSelector((state: GlobalState) => Boolean(getCallsAPI()?.selectors.isTranscriptionAvailable?.(state)));
}

export function useTranscription(channelId: string): CallsTranscription {
    return useSelector((state: GlobalState) => getCallsAPI()?.selectors.getTranscription?.(state, channelId) || NO_TRANSCRIPTION, shallowEqual);
}

function useLiveCaptions(channelId: string): CallsCaption[] {
    return useSelector((state: GlobalState) => getCallsAPI()?.selectors.getLiveCaptions?.(state, channelId) || NO_CAPTIONS);
}

// Whether to show the live captions: the user's choice, kept in this browser, shared by every view of the call.
const CAPTIONS_KEY = 'antimatter.calls.showCaptions';
const captionListeners = new Set<() => void>();
function readShowCaptions() {
    try {
        return localStorage.getItem(CAPTIONS_KEY) !== 'false';
    } catch {
        return true;
    }
}
let showCaptions = readShowCaptions();

export function useShowCaptions(): [boolean, (show: boolean) => void] {
    const show = useSyncExternalStore((cb) => {
        captionListeners.add(cb);
        return () => captionListeners.delete(cb);
    }, () => showCaptions);
    const set = (next: boolean) => {
        showCaptions = next;
        try {
            localStorage.setItem(CAPTIONS_KEY, String(next));
        } catch {
            // The choice lasts until the page reloads
        }
        captionListeners.forEach((cb) => cb());
    };
    return [show, set];
}

function Caption({caption}: {caption: CallsCaption}) {
    const name = useDisplayName(useUser(caption.userId));
    return (
        <div className={am('caption')}>
            <span className={am('caption-name')}>{name}</span>
            <span>{caption.text}</span>
        </div>
    );
}

// TranscriptionOverlay is drawn on the stage: the "transcribing" badge, and the captions at the bottom.
export function TranscriptionOverlay({channelId}: {channelId: string}) {
    const {formatMessage} = useIntl();
    const transcription = useTranscription(channelId);
    const captions = useLiveCaptions(channelId);
    const [show] = useShowCaptions();
    if (!transcription.running) {
        return null;
    }
    return (
        <>
            <div
                className={am('transcribing')}
                title={formatMessage({id: 'fusion.calls.transcribingTip', defaultMessage: 'What everyone says is transcribed; the transcript is posted in the call\'s thread when it stops.'})}
            >
                <i/>
                {formatMessage({id: 'fusion.calls.transcribing', defaultMessage: 'Transcribing'})}
            </div>
            {show && transcription.liveCaptions && captions.length > 0 && (
                <div
                    className={am('captions')}
                    aria-live='polite'
                >
                    {captions.slice(-3).map((c) => (
                        <Caption
                            key={c.captionId}
                            caption={c}
                        />
                    ))}
                </div>
            )}
        </>
    );
}

// TranscriptionControls are the call controls' buttons: transcribing (the host's) and showing the captions.
export function TranscriptionControls({channelId, isHost}: {channelId: string; isHost: boolean}) {
    const {formatMessage} = useIntl();
    const available = useTranscriptionAvailable();
    const transcription = useTranscription(channelId);
    const [show, setShow] = useShowCaptions();
    if (!available) {
        return null;
    }

    const toggle = () => {
        const api = getCallsAPI();
        const action = transcription.running ? api?.stopTranscription : api?.startTranscription;
        action?.().then(
            () => showToast(transcription.running ?
                formatMessage({id: 'fusion.calls.transcriptionStopped', defaultMessage: 'Transcription stopped: the transcript will be posted in the call\'s thread'}) :
                formatMessage({id: 'fusion.calls.transcriptionStarted', defaultMessage: 'Transcription started'})),
            () => showToast(formatMessage({id: 'fusion.calls.transcriptionFailed', defaultMessage: 'The transcription couldn\'t be changed'})),
        );
    };
    const transcribeLabel = transcription.running ?
        formatMessage({id: 'fusion.calls.stopTranscribing', defaultMessage: 'Stop transcribing'}) :
        formatMessage({id: 'fusion.calls.transcribe', defaultMessage: 'Transcribe the call'});
    const captionsLabel = show ?
        formatMessage({id: 'fusion.calls.hideCaptions', defaultMessage: 'Hide live captions'}) :
        formatMessage({id: 'fusion.calls.showCaptions', defaultMessage: 'Show live captions'});

    return (
        <>
            {isHost && (
                <button
                    className={am('round', {on: transcription.running})}
                    aria-label={transcribeLabel}
                    aria-pressed={transcription.running}
                    title={transcribeLabel}
                    onClick={toggle}
                >
                    <Icon name='transcript'/>
                </button>
            )}
            {transcription.liveCaptions && (
                <button
                    className={am('round', {on: show})}
                    aria-label={captionsLabel}
                    aria-pressed={show}
                    title={captionsLabel}
                    onClick={() => setShow(!show)}
                >
                    <Icon name='captions'/>
                </button>
            )}
        </>
    );
}
