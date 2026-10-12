// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState, useSyncExternalStore} from 'react';
import {useIntl} from 'react-intl';

import type {DesktopCaptureSource} from '@mattermost/desktop-api';

import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import {showToast} from 'fusion/components/toast';
import {am} from 'fusion/utils/class_names';
import DesktopApp from 'utils/desktop_api';

import {getCallsAPI} from './calls_api';
import type {CallsScreenQuality} from './plugin_api/calls_public_api';
import {screenPicker} from './screen_picker';
import type {ScreenSurface} from './screen_picker';
import {useCallsUI} from './ui_state';

// In the desktop app, sharing a screen starts with picking one of the screens and windows it lists. The Fusion UI
// shows them in a dialog of its own, in the window the call is shown in; Calls' own picker is left to the classic
// web UI.

type PickRequest = {quality?: CallsScreenQuality};

let request: PickRequest | null = null;
const listeners = new Set<() => void>();

function setRequest(next: PickRequest | null) {
    request = next;
    listeners.forEach((listener) => listener());
}

function subscribe(listener: () => void) {
    listeners.add(listener);
    return () => {
        listeners.delete(listener);
    };
}

// startScreenShare shares the screen of the call run by this window: through the desktop app's sources and the
// Fusion UI's picker in the desktop app, through the browser's own picker otherwise (call it right from the click).
export function startScreenShare(quality?: CallsScreenQuality, surface?: ScreenSurface) {
    const api = getCallsAPI();
    if (!api) {
        return;
    }
    if (screenPicker() === 'desktop' && DesktopApp.canGetDesktopSources()) {
        setRequest({quality});
        return;
    }
    api.startScreenShare(quality, surface);
}

// The desktop app's call window (the classic web UI's) lists itself as a window.
const HIDDEN_SOURCES = ['Calls Widget'];

const THUMBNAIL_SIZE = {width: 400, height: 400};

// Screens come first, then windows; the desktop app names source ids after their kind ("screen:1:0", "window:42:0").
export function groupSources(sources: DesktopCaptureSource[]) {
    const shown = sources.filter((s) => !HIDDEN_SOURCES.includes(s.name));
    return {
        screens: shown.filter((s) => s.id.startsWith('screen:')),
        windows: shown.filter((s) => !s.id.startsWith('screen:')),
    };
}

type Groups = ReturnType<typeof groupSources>;

function SourceTile({source, on, onPick, onShare}: {source: DesktopCaptureSource; on: boolean; onPick: () => void; onShare: () => void}) {
    return (
        <button
            type='button'
            role='radio'
            aria-checked={on}
            className={am('src', {on})}
            title={source.name}
            onClick={onPick}
            onDoubleClick={onShare}
        >
            <span className={am('src-thumb')}>
                <img
                    src={source.thumbnailURL}
                    alt=''
                    draggable={false}
                />
            </span>
            <span className={am('src-name')}>{source.name}</span>
        </button>
    );
}

// ScreenSourceDialog lists the screens and windows to share, in a grid of thumbnails, and shares the one picked.
export function ScreenSourceDialog({quality, onClose}: {quality?: CallsScreenQuality; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const [groups, setGroups] = useState<Groups | null>(null);
    const [selected, setSelected] = useState('');
    const [withAudio, setWithAudio] = useState(false);
    const canShareAudio = Boolean(getCallsAPI()?.canShareSystemAudio?.());

    useEffect(() => {
        let live = true;
        const failed = () => {
            if (live) {
                showToast(formatMessage({id: 'fusion.calls.sources.failed', defaultMessage: 'The screens and windows to share couldn\'t be listed'}));
                onClose();
            }
        };
        DesktopApp.getDesktopSources({types: ['screen', 'window'], thumbnailSize: THUMBNAIL_SIZE}).then((sources) => {
            const next = groupSources(sources);
            const first = next.screens[0] || next.windows[0];
            if (!first) {
                failed();
                return;
            }
            if (live) {
                setGroups(next);
                setSelected(first.id);
            }
        }, failed);
        return () => {
            live = false;
        };

    // The sources are listed once, when the dialog opens.
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, []);

    const share = (sourceId = selected) => {
        onClose();
        if (sourceId) {
            getCallsAPI()?.startScreenShare(quality, undefined, {desktopSourceId: sourceId, withAudio: canShareAudio && withAudio});
        }
    };

    const title = formatMessage({id: 'fusion.calls.sources.title', defaultMessage: 'Choose what to share'});
    const section = (label: string, sources: DesktopCaptureSource[]) => sources.length > 0 && (
        <section>
            <h3 className={am('src-head')}>{label}</h3>
            <div
                className={am('src-grid')}
                role='radiogroup'
                aria-label={label}
            >
                {sources.map((source) => (
                    <SourceTile
                        key={source.id}
                        source={source}
                        on={source.id === selected}
                        onPick={() => setSelected(source.id)}
                        onShare={() => share(source.id)}
                    />
                ))}
            </div>
        </section>
    );

    return (
        <Dialog
            label={title}
            onClose={onClose}
            small={false}
            className={am('src-dialog')}
        >
            <div className={am('src-top')}>
                <h2>{title}</h2>
                <button
                    type='button'
                    className={am('icon-btn')}
                    aria-label={formatMessage({id: 'fusion.calls.sources.close', defaultMessage: 'Close'})}
                    onClick={onClose}
                >
                    <Icon name='x'/>
                </button>
            </div>
            <div className={am('src-body')}>
                {groups ? (
                    <>
                        {section(formatMessage({id: 'fusion.calls.sources.screens', defaultMessage: 'Screens'}), groups.screens)}
                        {section(formatMessage({id: 'fusion.calls.sources.windows', defaultMessage: 'Windows'}), groups.windows)}
                    </>
                ) : (
                    <p className={am('src-loading')}>
                        {formatMessage({id: 'fusion.calls.sources.loading', defaultMessage: 'Looking for screens and windows…'})}
                    </p>
                )}
            </div>
            <div className={am('src-foot')}>
                {canShareAudio && (
                    <label className={am('src-audio')}>
                        <button
                            type='button'
                            className={am('switch')}
                            role='switch'
                            aria-checked={withAudio}
                            onClick={() => setWithAudio(!withAudio)}
                        />
                        {formatMessage({id: 'fusion.calls.sources.audio', defaultMessage: 'Also share the computer\'s sound'})}
                    </label>
                )}
                <span className={am('grow')}/>
                <button
                    type='button'
                    className={am('btn')}
                    onClick={onClose}
                >
                    {formatMessage({id: 'fusion.calls.sources.cancel', defaultMessage: 'Cancel'})}
                </button>
                <button
                    type='button'
                    className={am('btn', 'primary')}
                    disabled={!selected}
                    onClick={() => share()}
                >
                    {formatMessage({id: 'fusion.calls.sources.share', defaultMessage: 'Share'})}
                </button>
            </div>
        </Dialog>
    );
}

// ScreenSourcePicker shows the picker once asked for (see startScreenShare): in the call's own window when it's
// popped out (inPopout), in the main window otherwise.
export function ScreenSourcePicker({inPopout = false}: {inPopout?: boolean}) {
    const current = useSyncExternalStore(subscribe, () => request);
    const {callPopout} = useCallsUI();
    if (!current || inPopout !== Boolean(callPopout)) {
        return null;
    }
    return (
        <ScreenSourceDialog
            quality={current.quality}
            onClose={() => setRequest(null)}
        />
    );
}
