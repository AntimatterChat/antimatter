// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import ReactDOM from 'react-dom';
import {useIntl} from 'react-intl';

import {IconSprite} from 'fusion/components/icon';
import {LayerRoot} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

import {useChannelLabel} from './actions';
import CallControls from './controls';
import {useMyCall} from './hooks';
import Stage from './stage';
import {closeCallPopout, useCallsUI} from './ui_state';

// copyLook gives a new window the page's styles, theme and fonts.
function copyLook(from: Document, to: Document) {
    from.querySelectorAll('link[rel="stylesheet"], style').forEach((node) => to.head.appendChild(node.cloneNode(true)));
    to.documentElement.className = from.documentElement.className;
    to.documentElement.style.cssText = from.documentElement.style.cssText;
    Object.entries(from.documentElement.dataset).forEach(([key, value]) => {
        to.documentElement.dataset[key] = value;
    });
    to.body.className = from.body.className;
}

// CallPopout shows the call (its stage and controls) in a browser window of its own: this page draws in it, so the
// call, its media and its controls stay this page's. Closing the window, or the call's end, brings it back.
export default function CallPopout() {
    const {formatMessage} = useIntl();
    const {callPopout} = useCallsUI();
    const my = useMyCall();
    const label = useChannelLabel(my?.call.channelId);
    const [target, setTarget] = useState<{root: HTMLElement; layer: HTMLElement} | null>(null);

    // The call over, the window goes
    useEffect(() => {
        if (callPopout && !my) {
            closeCallPopout();
        }
    }, [callPopout, my]);

    useEffect(() => {
        if (!callPopout) {
            setTarget(null);
            return () => null;
        }
        const doc = callPopout.document;
        doc.body.replaceChildren();
        copyLook(document, doc);
        const root = doc.createElement('div');
        root.className = 'am-app am-call-popout';
        const layer = doc.createElement('div');
        layer.className = 'am-layer';
        doc.body.append(root, layer);
        setTarget({root, layer});

        const onClosed = () => closeCallPopout();
        callPopout.addEventListener('pagehide', onClosed);
        return () => callPopout.removeEventListener('pagehide', onClosed);
    }, [callPopout]);

    useEffect(() => {
        if (callPopout && my) {
            callPopout.document.title = my.voice ? label : formatMessage({id: 'fusion.calls.windowTitle', defaultMessage: 'Call · {where}'}, {where: label});
        }
    }, [callPopout, my, label, formatMessage]);

    if (!target || !my) {
        return null;
    }
    return ReactDOM.createPortal(
        <LayerRoot.Provider value={target.layer}>
            <IconSprite/>
            <div className={am('call-pop-body')}>
                <Stage channelId={my.call.channelId}/>
                <CallControls my={my}/>
            </div>
        </LayerRoot.Provider>,
        target.root,
    );
}
