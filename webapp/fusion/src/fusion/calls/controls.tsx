// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch} from 'react-redux';

import {selectPostById} from 'actions/views/rhs';

import RenderEmoji from 'components/emoji/render_emoji';

import Icon from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {MenuHeading, MenuItem, MenuSeparator} from 'fusion/components/menu';
import {showToast} from 'fusion/components/toast';
import {am} from 'fusion/utils/class_names';

import {useCallActions} from './actions';
import {getCallsAPI} from './calls_api';
import {BLUR_INTENSITIES, blurPreset, getScreenQuality, QUICK_REACTIONS, SCREEN_FRAME_RATES, SCREEN_HEIGHTS, setScreenQuality, setScreenSurface} from './extras';
import {useCall} from './hooks';
import type {MyCallState} from './hooks';
import {screenPicker} from './screen_picker';
import type {ScreenSurface} from './screen_picker';
import {startScreenShare} from './screen_sources';
import {TranscriptionControls} from './transcription';
import {openCallPopout, useCallsUI} from './ui_state';

// useToggleLabels gives the labels shared by the voice panel's buttons and the round controls.
export function useToggleLabels(my: MyCallState | null) {
    const {formatMessage} = useIntl();
    let screen = formatMessage({id: 'fusion.calls.shareScreen', defaultMessage: 'Share screen'});
    if (my && !my.screenAllowed) {
        screen = formatMessage({id: 'fusion.calls.screenNotAllowed', defaultMessage: 'Screen sharing is turned off'});
    } else if (my?.otherSharing) {
        screen = formatMessage({id: 'fusion.calls.someoneSharing', defaultMessage: 'Someone else is sharing their screen'});
    }
    let muteLabel = my?.muted ? formatMessage({id: 'fusion.calls.unmute', defaultMessage: 'Unmute'}) : formatMessage({id: 'fusion.calls.mute', defaultMessage: 'Mute'});
    if (my?.modMuted) {
        muteLabel = formatMessage({id: 'fusion.calls.modMutedYou', defaultMessage: 'A moderator muted you'});
    }
    return {
        mute: muteLabel,
        deafen: formatMessage({id: 'fusion.calls.deafen', defaultMessage: 'Deafen'}),
        video: my?.videoAllowed === false ? formatMessage({id: 'fusion.calls.cameraNotAllowed', defaultMessage: 'Cameras are turned off in this call'}) : formatMessage({id: 'fusion.calls.camera', defaultMessage: 'Camera'}),
        screen,
        hand: my?.handRaised ? formatMessage({id: 'fusion.calls.lowerHand', defaultMessage: 'Lower hand'}) : formatMessage({id: 'fusion.calls.raiseHand', defaultMessage: 'Raise hand'}),
        popout: formatMessage({id: 'fusion.calls.popout', defaultMessage: 'Open in a new window'}),
        popoutLabel: formatMessage({id: 'fusion.calls.popoutLabel', defaultMessage: 'Open the call in a new window'}),
        show: my?.voice ? formatMessage({id: 'fusion.calls.showVoice', defaultMessage: 'Show the voice channel'}) : formatMessage({id: 'fusion.calls.showCall', defaultMessage: 'Show the call'}),
        disconnect: formatMessage({id: 'fusion.calls.disconnect', defaultMessage: 'Disconnect'}),
    };
}

// muteIcon is the microphone of the mute buttons: locked when a moderator muted the user.
export function muteIcon(my: MyCallState): 'mic' | 'mic-off' | 'mic-lock' {
    if (my.modMuted) {
        return 'mic-lock';
    }
    return my.muted ? 'mic-off' : 'mic';
}

// Chip is a preset of a menu (a resolution, a frame rate, a blur), the chosen one highlighted.
function Chip({on, label, onClick}: {on: boolean; label: string; onClick: () => void}) {
    return (
        <button
            type='button'
            className={am('q-chip', {on})}
            aria-pressed={on}
            onClick={onClick}
        >
            {label}
        </button>
    );
}

// ScreenShareMenu shares the screen: what, and at which size and frame rate (Vencord's presets; the choice is kept).
// What is offered follows the picker that opens (see screenPicker): Chromium opens its picker on a whole screen, a
// window or a browser tab; Firefox, Safari and the desktop app show their own picker, whatever is asked for.
export function ScreenShareMenu({anchor, placement = 'above', onClose}: {anchor: HTMLElement | null; placement?: 'above' | 'below'; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const [quality, setQuality] = useState(getScreenQuality);
    const [picker] = useState(() => screenPicker());

    // Sharing must start right from the click: the browser asks only after a user's action. The choice is kept for the
    // share button.
    const share = (surface?: ScreenSurface) => {
        setScreenQuality(quality);
        setScreenSurface(surface);
        onClose();
        startScreenShare(quality, surface);
    };
    return (
        <Popover
            anchor={anchor}
            placement={placement}
            className='plus-pop q-pop'
            role='menu'
            label={formatMessage({id: 'fusion.calls.shareWhat', defaultMessage: 'Share your screen'})}
            onClose={onClose}
        >
            <MenuHeading>{formatMessage({id: 'fusion.calls.shareWhat', defaultMessage: 'Share your screen'})}</MenuHeading>
            {picker === 'surfaces' ? (
                <>
                    <MenuItem
                        icon='screen'
                        label={formatMessage({id: 'fusion.calls.shareMonitor', defaultMessage: 'A whole screen'})}
                        onClick={() => share('monitor')}
                    />
                    <MenuItem
                        icon='rect'
                        label={formatMessage({id: 'fusion.calls.shareWindow', defaultMessage: 'A window'})}
                        onClick={() => share('window')}
                    />
                    <MenuItem
                        icon='globe'
                        label={formatMessage({id: 'fusion.calls.shareTab', defaultMessage: 'A browser tab'})}
                        onClick={() => share('browser')}
                    />
                </>
            ) : (
                <MenuItem
                    icon='screen'
                    label={formatMessage({id: 'fusion.calls.shareSource', defaultMessage: 'Choose a screen or a window…'})}
                    onClick={() => share()}
                />
            )}
            <MenuSeparator/>
            <MenuHeading>{formatMessage({id: 'fusion.calls.resolution', defaultMessage: 'Resolution'})}</MenuHeading>
            <div className={am('q-chips')}>
                {SCREEN_HEIGHTS.map((h) => (
                    <Chip
                        key={h}
                        on={quality.height === h}
                        label={h ? `${h}p` : formatMessage({id: 'fusion.calls.source', defaultMessage: 'Source'})}
                        onClick={() => setQuality({...quality, height: h})}
                    />
                ))}
            </div>
            <MenuHeading>{formatMessage({id: 'fusion.calls.frameRate', defaultMessage: 'Frame rate'})}</MenuHeading>
            <div className={am('q-chips')}>
                {SCREEN_FRAME_RATES.map((f) => (
                    <Chip
                        key={f}
                        on={quality.frameRate === f}
                        label={`${f} fps`}
                        onClick={() => setQuality({...quality, frameRate: f})}
                    />
                ))}
            </div>
        </Popover>
    );
}

// CameraMenu sets the camera up, as the share menu does the screen: Calls' background blur and how strong (presets;
// Calls keeps them for the next calls), and the devices in the Calls settings.
export function CameraMenu({anchor, onClose}: {anchor: HTMLElement | null; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const api = getCallsAPI();
    const [blur, setBlur] = useState(() => api?.getBackgroundBlur?.() || {on: false, intensity: 50});
    const apply = (next: typeof blur) => {
        setBlur(next);
        api?.setBackgroundBlur?.(next).catch(() => showToast(formatMessage({id: 'fusion.calls.blurFailed', defaultMessage: 'The background couldn\'t be blurred'})));
    };
    const preset = blurPreset(blur.intensity);
    const presetLabels: Record<number, string> = {
        25: formatMessage({id: 'fusion.calls.blurLight', defaultMessage: 'Light'}),
        50: formatMessage({id: 'fusion.calls.blurMedium', defaultMessage: 'Medium'}),
        100: formatMessage({id: 'fusion.calls.blurStrong', defaultMessage: 'Strong'}),
    };
    return (
        <Popover
            anchor={anchor}
            placement='above'
            className='plus-pop q-pop'
            role='menu'
            label={formatMessage({id: 'fusion.calls.cameraOptions', defaultMessage: 'Camera options'})}
            onClose={onClose}
        >
            <MenuHeading>{formatMessage({id: 'fusion.calls.camera', defaultMessage: 'Camera'})}</MenuHeading>
            {api?.setBackgroundBlur && (
                <MenuItem
                    icon='eye-off'
                    role='menuitemcheckbox'
                    checked={blur.on}
                    label={formatMessage({id: 'fusion.calls.blur', defaultMessage: 'Blur my background'})}
                    onClick={() => apply({on: !blur.on, intensity: preset})}
                />
            )}
            {api?.setBackgroundBlur && blur.on && (
                <>
                    <MenuHeading>{formatMessage({id: 'fusion.calls.blurAmount', defaultMessage: 'Amount'})}</MenuHeading>
                    <div className={am('q-chips')}>
                        {BLUR_INTENSITIES.map((i) => (
                            <Chip
                                key={i}
                                on={preset === i}
                                label={presetLabels[i]}
                                onClick={() => apply({on: true, intensity: i})}
                            />
                        ))}
                    </div>
                </>
            )}
            <MenuSeparator/>
            <MenuItem
                icon='cog'
                label={formatMessage({id: 'fusion.calls.devices', defaultMessage: 'Camera and microphone settings…'})}
                onClick={() => {
                    onClose();
                    api?.openSettings();
                }}
            />
        </Popover>
    );
}

// ReactionsMenu sends a reaction to the call, shown over the stage for everyone.
function ReactionsMenu({anchor, onClose}: {anchor: HTMLElement | null; onClose: () => void}) {
    const {formatMessage} = useIntl();
    return (
        <Popover
            anchor={anchor}
            placement='above-center'
            className='plus-pop react-pop'
            label={formatMessage({id: 'fusion.calls.react', defaultMessage: 'React'})}
            onClose={onClose}
        >
            <div className={am('quick-react')}>
                {QUICK_REACTIONS.map(([name, unified]) => (
                    <button
                        key={name}
                        type='button'
                        aria-label={formatMessage({id: 'fusion.messageMenu.reactWith', defaultMessage: 'React with :{name}:'}, {name})}
                        onClick={() => getCallsAPI()?.sendReaction?.(name, unified)}
                    >
                        <RenderEmoji
                            emojiName={name}
                            size={22}
                        />
                    </button>
                ))}
            </div>
        </Popover>
    );
}

type Props = {
    my: MyCallState;

    // The voice channel view offers to open the call in its window; the window itself doesn't.
    withPopout?: boolean;
};

// CallControls is the mockup's row of round call controls, under the stage. Deafen exists in voice channels only;
// when the call runs in another window (the desktop app's), only leaving can be done from here.
export default function CallControls({my, withPopout = false}: Props) {
    const {callPopout} = useCallsUI();
    const actions = useCallActions();
    const labels = useToggleLabels(my);
    const media = my.call.mediaInThisWindow;
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const call = useCall(my.call.channelId);
    const api = getCallsAPI();
    const [menu, setMenu] = useState<'screen' | 'camera' | 'react' | null>(null);
    const screenRef = useRef<HTMLSpanElement>(null);
    const cameraRef = useRef<HTMLSpanElement>(null);
    const reactRef = useRef<HTMLButtonElement>(null);

    // The options of a button open from a small badge on its top-right corner; the menu is placed by the button and
    // its badge together, so that clicking either isn't clicking away from it.
    const badge = (which: 'camera' | 'screen', label: string) => (
        <button
            className={am('round-badge', {on: menu === which})}
            aria-label={label}
            title={label}
            aria-haspopup='menu'
            aria-expanded={menu === which}
            onClick={() => setMenu(menu === which ? null : which)}
        >
            <Icon
                name='chev'
                size='xs'
            />
        </button>
    );
    const sharable = my.screenAllowed && !my.otherSharing;

    return (
        <div className={am('voice-controls')}>
            {media && (
                <>
                    <button
                        className={am('round', {off: my.muted})}
                        aria-label={labels.mute}
                        aria-pressed={my.muted}
                        title={my.modMuted ? labels.mute : undefined}
                        onClick={() => actions.toggleMute(my)}
                    >
                        <Icon name={muteIcon(my)}/>
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
                    <span
                        ref={cameraRef}
                        className={am('round-wrap')}
                    >
                        <button
                            className={am('round', {on: my.video})}
                            aria-label={labels.video}
                            aria-pressed={my.video}
                            title={my.videoAllowed ? undefined : labels.video}
                            disabled={!my.videoAllowed && !my.video}
                            onClick={() => {
                                setMenu(null);
                                actions.toggleVideo(my);
                            }}
                        >
                            <Icon name='video'/>
                        </button>
                        {api && badge('camera', formatMessage({id: 'fusion.calls.cameraOptions', defaultMessage: 'Camera options'}))}
                    </span>
                    <span
                        ref={screenRef}
                        className={am('round-wrap')}
                    >
                        <button
                            className={am('round', {on: my.screen})}
                            aria-label={my.screen ? formatMessage({id: 'fusion.calls.stopSharing', defaultMessage: 'Stop sharing'}) : labels.screen}
                            aria-pressed={my.screen}
                            title={my.screen || sharable ? undefined : labels.screen}
                            disabled={!my.screen && !sharable}
                            onClick={() => {
                                setMenu(null);
                                actions.toggleScreen(my);
                            }}
                        >
                            <Icon name='screen'/>
                        </button>

                        {/* What, at which size and frame rate: for the next share, so not while sharing. */}
                        {api && !my.screen && sharable && badge('screen', formatMessage({id: 'fusion.calls.shareOptions', defaultMessage: 'Screen share options'}))}
                    </span>
                    <button
                        className={am('round', {on: my.handRaised})}
                        aria-label={labels.hand}
                        aria-pressed={my.handRaised}
                        title={labels.hand}
                        onClick={() => actions.toggleHand(my)}
                    >
                        <Icon name='hand'/>
                    </button>
                    <TranscriptionControls
                        channelId={my.call.channelId}
                        isHost={my.isHost}
                    />
                    {api?.sendReaction && (
                        <button
                            ref={reactRef}
                            className={am('round', {on: menu === 'react'})}
                            aria-label={formatMessage({id: 'fusion.calls.react', defaultMessage: 'React'})}
                            title={formatMessage({id: 'fusion.calls.react', defaultMessage: 'React'})}
                            onClick={() => setMenu(menu === 'react' ? null : 'react')}
                        >
                            <Icon name='smile'/>
                        </button>
                    )}
                </>
            )}
            {!my.voice && call?.threadId && (
                <button
                    className={am('round')}
                    aria-label={formatMessage({id: 'fusion.calls.callChat', defaultMessage: 'Call chat'})}
                    title={formatMessage({id: 'fusion.calls.callChat', defaultMessage: 'Call chat'})}
                    onClick={() => dispatch(selectPostById(call.threadId))}
                >
                    <Icon name='chat'/>
                </button>
            )}
            {menu === 'screen' && (
                <ScreenShareMenu
                    anchor={screenRef.current}
                    onClose={() => setMenu(null)}
                />
            )}
            {menu === 'camera' && (
                <CameraMenu
                    anchor={cameraRef.current}
                    onClose={() => setMenu(null)}
                />
            )}
            {menu === 'react' && (
                <ReactionsMenu
                    anchor={reactRef.current}
                    onClose={() => setMenu(null)}
                />
            )}
            {withPopout && !callPopout && (
                <button
                    className={am('round')}
                    title={labels.popout}
                    aria-label={labels.popoutLabel}
                    onClick={openCallPopout}
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
