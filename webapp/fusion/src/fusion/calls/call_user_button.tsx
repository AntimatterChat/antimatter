// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import {useCallActions} from './actions';
import {useCallsAvailable} from './hooks';

// useCanCall tells whether someone can be called from their profile: not yourself, not a bot, and only with Calls.
export function useCanCall(isMe: boolean, bot: boolean): boolean {
    return useCallsAvailable() && !isMe && !bot;
}

// CallUserButton is the green Call button of a profile card, beside Message: it opens your direct message with them
// and starts (or joins) its call.
export default function CallUserButton({userId, onDone}: {userId: string; onDone: () => void}) {
    const {formatMessage} = useIntl();
    const actions = useCallActions();
    return (
        <button
            className={am('btn')}
            style={{justifyContent: 'center', background: 'var(--am-ok)', color: '#fff'}}
            onClick={() => {
                onDone();
                actions.callUser(userId);
            }}
        >
            <Icon
                name='phone'
                size='sm'
            />
            {formatMessage({id: 'fusion.calls.call', defaultMessage: 'Call'})}
        </button>
    );
}
