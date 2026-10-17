// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {useCallActions} from './actions';
import {isRecordingAvailable} from './host_api';
import {useCall, useIsVoiceChannel} from './hooks';

// Recordings of calls, by the Calls plugin's job service (Enable call recordings): the host starts and stops them,
// everyone sees that the call is recorded, and the recording is posted in the call's thread once it stops. Calls
// in voice channels have no thread, so no recordings.

export function useRecordingAvailable(): boolean {
    return useSelector((state: GlobalState) => isRecordingAvailable(state));
}

// RecordingControl is the call controls' button to record the call, for its host.
export function RecordingControl({channelId, isHost}: {channelId: string; isHost: boolean}) {
    const {formatMessage} = useIntl();
    const available = useRecordingAvailable();
    const call = useCall(channelId);
    const voice = useIsVoiceChannel(channelId);
    const actions = useCallActions();
    const [busy, setBusy] = useState(false);
    if (!available || !isHost || voice || !call?.threadId) {
        return null;
    }

    const recording = Boolean(call.recording);
    const label = recording ?
        formatMessage({id: 'fusion.calls.stopRecording', defaultMessage: 'Stop recording'}) :
        formatMessage({id: 'fusion.calls.record', defaultMessage: 'Record the call'});
    return (
        <button
            className={am('round', {on: recording})}
            aria-label={label}
            aria-pressed={recording}
            title={label}
            disabled={busy}
            onClick={async () => {
                setBusy(true);
                try {
                    await actions.toggleRecording(channelId, recording);
                } finally {
                    setBusy(false);
                }
            }}
        >
            <Icon name='ellipse'/>
        </button>
    );
}

// RecordingBadge is drawn on the stage while the call is recorded, for everyone in it.
export function RecordingBadge({channelId}: {channelId: string}) {
    const {formatMessage} = useIntl();
    const call = useCall(channelId);
    if (!call?.recording) {
        return null;
    }
    return (
        <div
            className={am('transcribing', 'recording-badge')}
            title={formatMessage({id: 'fusion.calls.recordingTip', defaultMessage: 'This call is recorded; the recording is posted in the call\'s thread when it stops.'})}
        >
            <i/>
            {formatMessage({id: 'fusion.calls.recording', defaultMessage: 'Recording'})}
        </div>
    );
}
