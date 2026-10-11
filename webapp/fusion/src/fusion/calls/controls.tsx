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
import {getScreenQuality, QUICK_REACTIONS, SCREEN_FRAME_RATES, SCREEN_HEIGHTS, setScreenQuality} from './extras';
import {useCall} from './hooks';
import type {MyCallState} from './hooks';
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

type ScreenSurface = 'monitor' | 'window' | 'browser';

// ScreenShareMenu shares the screen: what (the whole screen, a window or a browser tab, which the browser's picker
// opens on), at which size and frame rate (Vencord's presets; the choice is kept). The desktop app has its own picker.
export function ScreenShareMenu({anchor, placement = 'above', onClose}: {anchor: HTMLElement | null; placement?: 'above' | 'below'; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const [quality, setQuality] = useState(getScreenQuality);

    // Sharing must start right from the click: the browser asks only after a user's action.
    const share = (surface?: ScreenSurface) => {
        setScreenQuality(quality);
        onClose();
        getCallsAPI()?.startScreenShare(quality, surface);
    };
    const chip = (on: boolean, label: string, onClick: () => void) => (
        <button
            key={label}
            type='button'
            className={am('q-chip', {on})}
            aria-pressed={on}
            onClick={onClick}
        >
            {label}
        </button>
    );
    const desktop = Boolean((window as {desktop?: unknown}).desktop);
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
            {desktop ? (
                <MenuItem
                    icon='screen'
                    label={formatMessage({id: 'fusion.calls.shareSource', defaultMessage: 'Choose a screen or a window…'})}
                    onClick={() => share()}
                />
            ) : (
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
            )}
            <MenuSeparator/>
            <MenuHeading>{formatMessage({id: 'fusion.calls.resolution', defaultMessage: 'Resolution'})}</MenuHeading>
            <div className={am('q-chips')}>
                {SCREEN_HEIGHTS.map((h) => chip(quality.height === h, h ? `${h}p` : formatMessage({id: 'fusion.calls.source', defaultMessage: 'Source'}), () => setQuality({...quality, height: h})))}
            </div>
            <MenuHeading>{formatMessage({id: 'fusion.calls.frameRate', defaultMessage: 'Frame rate'})}</MenuHeading>
            <div className={am('q-chips')}>
                {SCREEN_FRAME_RATES.map((f) => chip(quality.frameRate === f, `${f} fps`, () => setQuality({...quality, frameRate: f})))}
            </div>
        </Popover>
    );
}

// BlurMenu blurs the background of your camera (Calls' own background blur), and sets how much.
function BlurMenu({anchor, onClose}: {anchor: HTMLElement | null; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const api = getCallsAPI();
    const [blur, setBlur] = useState(() => api?.getBackgroundBlur?.() || {on: false, intensity: 50});
    const apply = (next: typeof blur) => {
        setBlur(next);
        api?.setBackgroundBlur?.(next).catch(() => showToast(formatMessage({id: 'fusion.calls.blurFailed', defaultMessage: 'The background couldn\'t be blurred'})));
    };
    return (
        <Popover
            anchor={anchor}
            placement='above'
            className='plus-pop q-pop'
            label={formatMessage({id: 'fusion.calls.camera', defaultMessage: 'Camera'})}
            onClose={onClose}
        >
            <MenuItem
                icon='eye-off'
                role='menuitemcheckbox'
                checked={blur.on}
                label={formatMessage({id: 'fusion.calls.blur', defaultMessage: 'Blur my background'})}
                onClick={() => apply({...blur, on: !blur.on})}
            />
            {blur.on && (
                <label className={am('vol-row')}>
                    <span>{formatMessage({id: 'fusion.calls.blurAmount', defaultMessage: 'Amount'})}</span>
                    <input
                        type='range'
                        min={10}
                        max={100}
                        step={10}
                        value={blur.intensity}
                        onChange={(e) => setBlur({...blur, intensity: Number(e.target.value)})}
                        onPointerUp={() => apply(blur)}
                        onKeyUp={() => apply(blur)}
                    />
                </label>
            )}
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
            placement='above'
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
    const screenRef = useRef<HTMLButtonElement>(null);
    const cameraRef = useRef<HTMLButtonElement>(null);
    const reactRef = useRef<HTMLButtonElement>(null);
    const caret = (which: 'camera', ref: React.RefObject<HTMLButtonElement | null>, label: string) => (
        <button
            ref={ref}
            className={am('round-caret', {on: menu === which})}
            aria-label={label}
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
                    {api?.setBackgroundBlur && caret('camera', cameraRef, formatMessage({id: 'fusion.calls.cameraOptions', defaultMessage: 'Camera options'}))}
                    <button
                        ref={screenRef}
                        className={am('round', {on: my.screen || menu === 'screen'})}
                        aria-label={my.screen ? formatMessage({id: 'fusion.calls.stopSharing', defaultMessage: 'Stop sharing'}) : labels.screen}
                        aria-pressed={my.screen}
                        aria-haspopup={my.screen ? undefined : 'menu'}
                        title={my.screen || (my.screenAllowed && !my.otherSharing) ? undefined : labels.screen}
                        disabled={!my.screen && (!my.screenAllowed || my.otherSharing)}
                        onClick={() => (my.screen ? actions.toggleScreen(my) : setMenu(menu === 'screen' ? null : 'screen'))}
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
                    aria-label={formatMessage({id: 'fusion.calls.chat', defaultMessage: 'Call chat'})}
                    title={formatMessage({id: 'fusion.calls.chat', defaultMessage: 'Call chat'})}
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
                <BlurMenu
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
