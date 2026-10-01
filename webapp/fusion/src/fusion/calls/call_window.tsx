// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';

import Icon from 'fusion/components/icon';
import {Layer} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

import {useChannelLabel} from './actions';
import CallControls from './controls';
import {useMyCall} from './hooks';
import Stage from './stage';
import {setCallWindowOpen, useCallsUI} from './ui_state';

// CallWindow is the mockup's call window: the call's stage and controls in a floating window that can be dragged by
// its bar, over whatever is on screen. It closes by its ✕ (back to the sidebar's panel) or when the call ends.
export default function CallWindow() {
    const {formatMessage} = useIntl();
    const {callWindow} = useCallsUI();
    const my = useMyCall();
    const label = useChannelLabel(my?.call.channelId);
    const [position, setPosition] = useState(() => ({left: Math.max(8, (window.innerWidth / 2) - 280), top: 90}));
    const [dragging, setDragging] = useState(false);
    const drag = useRef<{dx: number; dy: number} | null>(null);

    const open = callWindow && Boolean(my);
    useEffect(() => {
        if (callWindow && !my) {
            setCallWindowOpen(false);
        }
    }, [callWindow, my]);

    // A new window opens in the middle again, as the mockup's.
    useEffect(() => {
        if (open) {
            setPosition({left: Math.max(8, (window.innerWidth / 2) - 280), top: 90});
        }
    }, [open]);

    if (!open || !my) {
        return null;
    }

    const onPointerDown = (e: React.PointerEvent<HTMLDivElement>) => {
        if ((e.target as HTMLElement).closest('button')) {
            return;
        }
        drag.current = {dx: e.clientX - position.left, dy: e.clientY - position.top};
        e.currentTarget.setPointerCapture(e.pointerId);
        setDragging(true);
    };
    const onPointerMove = (e: React.PointerEvent<HTMLDivElement>) => {
        if (!drag.current) {
            return;
        }
        setPosition({
            left: Math.max(0, Math.min(e.clientX - drag.current.dx, window.innerWidth - 80)),
            top: Math.max(0, Math.min(e.clientY - drag.current.dy, window.innerHeight - 40)),
        });
    };
    const onPointerUp = () => {
        drag.current = null;
        setDragging(false);
    };

    const title = my.voice ? label : formatMessage({id: 'fusion.calls.windowTitle', defaultMessage: 'Call · {where}'}, {where: label});

    return (
        <Layer>
            <div
                className={am('float-win', 'call-win')}
                role='dialog'
                aria-label={formatMessage({id: 'fusion.calls.window', defaultMessage: 'Call window'})}
                style={{left: position.left, top: position.top}}
            >
                <div
                    className={am('float-bar', {dragging})}
                    onPointerDown={onPointerDown}
                    onPointerMove={onPointerMove}
                    onPointerUp={onPointerUp}
                    onPointerCancel={onPointerUp}
                >
                    <Icon
                        name={my.voice ? 'speaker' : 'phone'}
                        size='sm'
                    />
                    <b>{title}</b>
                    <button
                        className={am('icon-btn')}
                        title={formatMessage({id: 'fusion.calls.windowBack', defaultMessage: 'Back to the sidebar'})}
                        aria-label={formatMessage({id: 'fusion.calls.windowClose', defaultMessage: 'Close the call window'})}
                        onClick={() => setCallWindowOpen(false)}
                    >
                        <Icon
                            name='x'
                            size='sm'
                        />
                    </button>
                </div>
                <div className={am('float-body')}>
                    <Stage channelId={my.call.channelId}/>
                    <CallControls my={my}/>
                </div>
            </div>
        </Layer>
    );
}
