// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import {useCallActions} from './actions';
import type {MyCallState} from './hooks';
import {setCallWindowOpen} from './ui_state';

// useToggleLabels gives the labels shared by the voice panel's buttons and the round controls.
export function useToggleLabels(my: MyCallState | null) {
    const {formatMessage} = useIntl();
    let screen = formatMessage({id: 'fusion.calls.shareScreen', defaultMessage: 'Share screen'});
    if (my && !my.screenAllowed) {
        screen = formatMessage({id: 'fusion.calls.screenNotAllowed', defaultMessage: 'Screen sharing is turned off'});
    } else if (my?.otherSharing) {
        screen = formatMessage({id: 'fusion.calls.someoneSharing', defaultMessage: 'Someone else is sharing their screen'});
    }
    return {
        mute: my?.muted ? formatMessage({id: 'fusion.calls.unmute', defaultMessage: 'Unmute'}) : formatMessage({id: 'fusion.calls.mute', defaultMessage: 'Mute'}),
        deafen: formatMessage({id: 'fusion.calls.deafen', defaultMessage: 'Deafen'}),
        video: my?.videoAllowed === false ? formatMessage({id: 'fusion.calls.cameraNotAllowed', defaultMessage: 'Cameras are turned off in this call'}) : formatMessage({id: 'fusion.calls.camera', defaultMessage: 'Camera'}),
        screen,
        hand: my?.handRaised ? formatMessage({id: 'fusion.calls.lowerHand', defaultMessage: 'Lower hand'}) : formatMessage({id: 'fusion.calls.raiseHand', defaultMessage: 'Raise hand'}),
        popout: formatMessage({id: 'fusion.calls.popout', defaultMessage: 'Open in a new window'}),
        popoutLabel: formatMessage({id: 'fusion.calls.popoutLabel', defaultMessage: 'Open the call in a new window'}),
        disconnect: formatMessage({id: 'fusion.calls.disconnect', defaultMessage: 'Disconnect'}),
    };
}

type Props = {
    my: MyCallState;

    // The voice channel view offers to open the call in its window; the window itself doesn't.
    withPopout?: boolean;
};

// CallControls is the mockup's row of round call controls, under the stage. Deafen exists in voice channels only;
// when the call runs in another window (the desktop app's), only leaving can be done from here.
export default function CallControls({my, withPopout = false}: Props) {
    const actions = useCallActions();
    const labels = useToggleLabels(my);
    const media = my.call.mediaInThisWindow;

    return (
        <div className={am('voice-controls')}>
            {media && (
                <>
                    <button
                        className={am('round', {off: my.muted})}
                        aria-label={labels.mute}
                        aria-pressed={my.muted}
                        onClick={() => actions.toggleMute(my)}
                    >
                        <Icon name={my.muted ? 'mic-off' : 'mic'}/>
                    </button>
                    {my.voice && (
                        <button
                            className={am('round', {off: my.deafened})}
                            aria-label={labels.deafen}
                            aria-pressed={my.deafened}
                            onClick={() => actions.toggleDeafen(my)}
                        >
                            <Icon name={my.deafened ? 'headphones-off' : 'headphones'}/>
                        </button>
                    )}
                    <button
                        className={am('round', {on: my.video})}
                        aria-label={labels.video}
                        aria-pressed={my.video}
                        title={my.videoAllowed ? undefined : labels.video}
                        disabled={!my.videoAllowed && !my.video}
                        onClick={() => actions.toggleVideo(my)}
                    >
                        <Icon name='video'/>
                    </button>
                    <button
                        className={am('round', {on: my.screen})}
                        aria-label={labels.screen}
                        aria-pressed={my.screen}
                        title={my.screen || (my.screenAllowed && !my.otherSharing) ? undefined : labels.screen}
                        disabled={!my.screen && (!my.screenAllowed || my.otherSharing)}
                        onClick={() => actions.toggleScreen(my)}
                    >
                        <Icon name='screen'/>
                    </button>
                    <button
                        className={am('round', {on: my.handRaised})}
                        aria-label={labels.hand}
                        aria-pressed={my.handRaised}
                        title={labels.hand}
                        onClick={() => actions.toggleHand(my)}
                    >
                        <Icon name='hand'/>
                    </button>
                </>
            )}
            {withPopout && (
                <button
                    className={am('round')}
                    title={labels.popout}
                    aria-label={labels.popoutLabel}
                    onClick={() => setCallWindowOpen(true)}
                >
                    <Icon name='popout'/>
                </button>
            )}
            <button
                className={am('round', 'hang')}
                aria-label={labels.disconnect}
                onClick={actions.leaveCall}
            >
                <Icon name='hangup'/>
            </button>
        </div>
    );
}
