// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {useEffect, useRef} from 'react';
import {useIntl} from 'react-intl';

import {showToast} from 'fusion/components/toast';

import {useCallActions} from './actions';
import {useCallsAPIs} from './calls_api';
import type {CallsErrorEvent} from './calls_api';
import {useMyCall} from './hooks';
import {setCallWindowOpen, useCallsUI} from './ui_state';

type Shortcut = 'mute' | 'hand' | 'screen' | 'window' | 'leave';

function isMac() {
    return navigator.platform.toUpperCase().includes('MAC');
}

// The in-call shortcuts of Calls' widget, with the same keys: Ctrl (⌘ on macOS) + Shift + Space mutes, + Y raises a
// hand, + E shares the screen, + L leaves, and + P (or Alt + P) shows the call, here in its window.
function shortcutOf(e: KeyboardEvent): Shortcut | null {
    if (!e.key || !e.code) {
        return null;
    }
    const mod = isMac() ? 'meta+' : 'ctrl+';
    const mappings: Record<string, Shortcut> = {
        [mod + 'shift+ ']: 'mute',
        [mod + 'shift+space']: 'mute',
        [mod + 'shift+y']: 'hand',
        [mod + 'shift+e']: 'screen',
        [mod + 'shift+l']: 'leave',
        [mod + 'shift+p']: 'window',
        'alt+p': 'window',
    };
    const prefix = `${e.metaKey ? 'meta+' : ''}${e.ctrlKey ? 'ctrl+' : ''}${e.shiftKey ? 'shift+' : ''}${e.altKey ? 'alt+' : ''}`;

    // The key first, for other keyboard layouts, then the key's place on the keyboard.
    return mappings[prefix + e.key.toLowerCase()] || mappings[prefix + e.code.replace('Key', '').toLowerCase()] || null;
}

// CallEvents handles what the Calls widget did besides drawing: the in-call keyboard shortcuts, and telling about
// the call's device, permission and quality problems. It renders nothing.
export default function CallEvents() {
    const {formatMessage} = useIntl();
    const {calls} = useCallsAPIs();
    const actions = useCallActions();
    const my = useMyCall();
    const {callWindow} = useCallsUI();
    const latest = useRef({my, callWindow});
    latest.current = {my, callWindow};

    const active = Boolean(my && my.call.state === 'connected');
    useEffect(() => {
        if (!active) {
            return undefined;
        }
        const onKeyDown = (e: KeyboardEvent) => {
            const shortcut = shortcutOf(e);
            const current = latest.current.my;
            if (!shortcut || !current) {
                return;
            }
            e.preventDefault();
            e.stopImmediatePropagation();
            const media = current.call.mediaInThisWindow;
            if (shortcut === 'mute' && media) {
                actions.toggleMute(current);
            } else if (shortcut === 'hand' && media) {
                actions.toggleHand(current);
            } else if (shortcut === 'screen' && media && current.screenAllowed && (current.screen || !current.otherSharing)) {
                actions.toggleScreen(current);
            } else if (shortcut === 'window') {
                setCallWindowOpen(!latest.current.callWindow);
            } else if (shortcut === 'leave') {
                actions.leaveCall();
            }
        };
        document.addEventListener('keydown', onKeyDown, true);
        return () => document.removeEventListener('keydown', onKeyDown, true);
    }, [active, actions]);

    useEffect(() => {
        if (!calls) {
            return undefined;
        }
        return calls.on('error', (event: CallsErrorEvent) => {
            let text = '';
            switch (event.kind) {
            case 'mic-permissions':
                text = formatMessage({id: 'fusion.calls.error.micPermissions', defaultMessage: 'Allow access to your microphone to talk in the call'});
                break;
            case 'mic-missing':
                text = formatMessage({id: 'fusion.calls.error.micMissing', defaultMessage: 'No microphone found: others can\'t hear you'});
                break;
            case 'camera-permissions':
                text = formatMessage({id: 'fusion.calls.error.cameraPermissions', defaultMessage: 'Allow access to your camera to turn it on'});
                break;
            case 'camera-missing':
                text = formatMessage({id: 'fusion.calls.error.cameraMissing', defaultMessage: 'No camera found'});
                break;
            case 'device-fallback':
                text = formatMessage({id: 'fusion.calls.error.deviceFallback', defaultMessage: 'Now using {device}'}, {device: event.device.label || formatMessage({id: 'fusion.calls.error.defaultDevice', defaultMessage: 'the default device'})});
                break;
            case 'screen-share-failed':
                text = formatMessage({id: 'fusion.calls.error.screenShare', defaultMessage: 'Your screen wasn\'t shared'});
                break;
            case 'degraded-quality':
                text = formatMessage({id: 'fusion.calls.error.quality', defaultMessage: 'Your connection is weak: others may hear you poorly'});
                break;
            }

            // The others need nothing: Calls shows its own error modal when the call ends because of an error.
            if (text) {
                showToast(text);
            }
        });
    }, [calls, formatMessage]);

    return null;
}
