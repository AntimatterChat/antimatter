// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {DateTime} from 'luxon';
import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import type {IntlShape} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {WebSocketEvents} from '@mattermost/client';
import type {WebSocketMessage} from '@mattermost/client';

import {revokeSession} from 'mattermost-redux/actions/users';
import {Client4} from 'mattermost-redux/client';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import webSocketClient from 'client/web_websocket_client';
import Icon from 'fusion/components/icon';
import {Layer} from 'fusion/components/layer';
import {useMyZone} from 'fusion/composer/schedule';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

// A sign-in to your account, as the server tells your other sessions (session_created): the session's id and what
// the sessions list shows of it, never its token.
export type SignIn = {
    session_id: string;
    create_at: number;
    platform: string;
    os: string;
    browser: string;
    mobile: boolean;
};

// How many warnings are shown at once; the others wait their turn (a count says how many), so that a burst of
// sign-ins can't cover the screen.
export const MAX_SHOWN = 3;

// describeDevice names the device that signed in: the app or browser, and the system it runs on.
export function describeDevice(intl: IntlShape, signIn: SignIn): string {
    const name = (signIn.browser || '').split('/')[0].trim();
    let app = name === 'Unknown' ? '' : name;
    if (signIn.mobile || name === 'Mobile App') {
        app = intl.formatMessage({id: 'fusion.signIn.mobileApp', defaultMessage: 'Mobile app', description: 'Sign-in warning: the device that signed in is the mobile app.'});
    } else if (name === 'Desktop App') {
        app = intl.formatMessage({id: 'fusion.signIn.desktopApp', defaultMessage: 'Desktop app', description: 'Sign-in warning: the device that signed in is the desktop app.'});
    }
    const system = signIn.os || (signIn.platform === 'Unknown' ? '' : signIn.platform);
    if (app && system) {
        return intl.formatMessage({id: 'fusion.signIn.device', defaultMessage: '{app} on {system}', description: 'Sign-in warning: the app or browser that signed in, and its operating system, e.g. "Firefox on Linux".'}, {app, system});
    }
    return app || system || intl.formatMessage({id: 'fusion.signIn.unknownDevice', defaultMessage: 'Unknown device', description: 'Sign-in warning: the device that signed in is not known.'});
}

function SignInCard({signIn, onItsMe, onDisconnect}: {signIn: SignIn; onItsMe: () => void; onDisconnect: () => Promise<boolean>}) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const zone = useMyZone();
    const [busy, setBusy] = useState(false);

    const device = describeDevice(intl, signIn);
    const today = DateTime.fromMillis(signIn.create_at, {zone}).hasSame(DateTime.now().setZone(zone), 'day');
    const time = intl.formatDate(signIn.create_at, today ? {hour: 'numeric', minute: '2-digit', timeZone: zone} : {weekday: 'short', hour: 'numeric', minute: '2-digit', timeZone: zone});
    const title = formatMessage({id: 'fusion.signIn.title', defaultMessage: 'New sign-in to your account', description: 'Sign-in warning, at the top of the screen: someone signed in to your account on another device.'});

    return (
        <div
            className={am('signin')}
            role='alertdialog'
            aria-label={`${title}: ${device}`}
        >
            <div className={am('signin-top')}>
                <Icon
                    name='shield'
                    size='sm'
                />
                <div className={am('grow')}>
                    <b>{title}</b>
                    <span className={am('sub')}>
                        {formatMessage({id: 'fusion.signIn.detail', defaultMessage: '{device} · {time}', description: 'Sign-in warning: the device that signed in, and when (a time, with the day when not today).'}, {device, time})}
                    </span>
                    <span className={am('hint')}>
                        {formatMessage({id: 'fusion.signIn.hint', defaultMessage: 'Not you? Disconnect it, then change your password.', description: 'Sign-in warning: what to do when it was not you who signed in.'})}
                    </span>
                </div>
            </div>
            <div className={am('signin-actions')}>
                <button
                    className={am('btn')}
                    disabled={busy}
                    onClick={onItsMe}
                >
                    {formatMessage({id: 'fusion.signIn.itsMe', defaultMessage: 'It\'s me', description: 'Sign-in warning button: the sign-in was you; dismisses the warning on all your devices.'})}
                </button>
                <button
                    className={am('btn', 'danger')}
                    disabled={busy}
                    onClick={async () => {
                        setBusy(true);
                        if (!await onDisconnect()) {
                            setBusy(false);
                        }
                    }}
                >
                    {formatMessage({id: 'fusion.signIn.disconnect', defaultMessage: 'Disconnect', description: 'Sign-in warning button: signs that session out of your account.'})}
                </button>
            </div>
        </div>
    );
}

// SignInWarnings warns you, at the top of the Fusion UI, when someone signs in to your account (on any device, your
// own phone included) while you're connected: the device, the app or browser, and when. "Disconnect" signs that
// session out (the revoke-session API); "It's me" dismisses the warning. Either way, on any of your devices, the
// server tells all your sessions (session_revoked, session_acknowledged), and the warning goes from every tab.
export default function SignInWarnings() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const userId = useSelector(getCurrentUserId);
    const [signIns, setSignIns] = useState<SignIn[]>([]);

    useEffect(() => {
        const onMessage = (message: WebSocketMessage) => {
            switch (message.event) {
            case WebSocketEvents.SessionCreated: {
                const signIn = message.data;
                if (!signIn?.session_id) {
                    return;
                }

                // The newest on top.
                setSignIns((list) => (list.some((s) => s.session_id === signIn.session_id) ? list : [signIn, ...list]));
                break;
            }
            case WebSocketEvents.SessionRevoked:
            case WebSocketEvents.SessionAcknowledged: {
                const id = message.data?.session_id;
                setSignIns((list) => (list.some((s) => s.session_id === id) ? list.filter((s) => s.session_id !== id) : list));
                break;
            }
            }
        };
        webSocketClient.addMessageListener(onMessage);
        return () => webSocketClient.removeMessageListener(onMessage);
    }, []);

    if (!signIns.length || !userId) {
        return null;
    }

    const handled = (id: string) => setSignIns((list) => list.filter((s) => s.session_id !== id));
    const itsMe = (id: string) => {
        handled(id);
        Client4.acknowledgeSession(userId, id).catch(() => {
            // Dismissed here all the same; the other devices keep it until handled there.
        });
    };
    const disconnect = async (id: string) => {
        const result = await dispatch(revokeSession(userId, id));
        if (result.error) {
            toast(formatMessage({id: 'fusion.signIn.disconnectFailed', defaultMessage: 'Couldn\'t disconnect that session. Try again.', description: 'Toast when signing out the session from a sign-in warning failed.'}));
            return false;
        }
        handled(id);
        return true;
    };

    const more = signIns.length - MAX_SHOWN;
    return (
        <Layer>
            <div className={am('signins')}>
                {signIns.slice(0, MAX_SHOWN).map((signIn) => (
                    <SignInCard
                        key={signIn.session_id}
                        signIn={signIn}
                        onItsMe={() => itsMe(signIn.session_id)}
                        onDisconnect={() => disconnect(signIn.session_id)}
                    />
                ))}
                {more > 0 && (
                    <span className={am('signins-more')}>
                        {formatMessage({id: 'fusion.signIn.more', defaultMessage: '{count, plural, one {# more sign-in} other {# more sign-ins}}', description: 'Under the sign-in warnings shown, how many more wait their turn.'}, {count: more})}
                    </span>
                )}
            </div>
        </Layer>
    );
}
