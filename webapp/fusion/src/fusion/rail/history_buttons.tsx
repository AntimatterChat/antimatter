// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useHistory} from 'react-router-dom';

import {isDesktopApp, isMac} from '@mattermost/shared/utils/user_agent';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';
import DesktopApp from 'utils/desktop_api';

// HistoryButtons go back and forward through the conversations opened, at the top of the server rail, in the desktop
// app only: it has no browser buttons of its own. The app tells when there's nothing more either way.
export default function HistoryButtons() {
    const {formatMessage} = useIntl();
    const history = useHistory();
    const [canGoBack, setCanGoBack] = useState(true);
    const [canGoForward, setCanGoForward] = useState(true);

    useEffect(() => DesktopApp.onBrowserHistoryStatusUpdated((back, forward) => {
        setCanGoBack(back);
        setCanGoForward(forward);
    }), []);

    const refresh = async () => {
        const status = await DesktopApp.getBrowserHistoryStatus();
        setCanGoBack(status.canGoBack);
        setCanGoForward(status.canGoForward);
    };

    const keys = isMac() ? {back: '⌘[', forward: '⌘]'} : {back: 'Alt+←', forward: 'Alt+→'};
    const back = formatMessage({id: 'fusion.history.back', defaultMessage: 'Back'});
    const forward = formatMessage({id: 'fusion.history.forward', defaultMessage: 'Forward'});
    return (
        <div className={am('history-btns')}>
            <button
                type='button'
                className={am('back')}
                title={`${back} (${keys.back})`}
                aria-label={back}
                disabled={!canGoBack}
                onClick={() => {
                    history.goBack();
                    refresh();
                }}
            >
                <Icon
                    name='chev'
                    size='sm'
                />
            </button>
            <button
                type='button'
                className={am('forward')}
                title={`${forward} (${keys.forward})`}
                aria-label={forward}
                disabled={!canGoForward}
                onClick={() => {
                    history.goForward();
                    refresh();
                }}
            >
                <Icon
                    name='chev'
                    size='sm'
                />
            </button>
        </div>
    );
}

export const showHistoryButtons = isDesktopApp;
