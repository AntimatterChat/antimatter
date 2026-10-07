// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useSelector} from 'react-redux';

import {makeGetUsersTypingByChannelAndPost} from 'mattermost-redux/selectors/entities/typing';

import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// How many people the line names before saying several people are typing.
const NAMED = 3;

// Typing tells who is typing in the conversation or thread, under the message box, as in Discord.
export default function Typing({channelId, rootId}: {channelId: string; rootId: string}) {
    const intl = useIntl();
    const getTyping = useMemo(() => makeGetUsersTypingByChannelAndPost(), []);
    const names = useSelector((state: GlobalState) => getTyping(state, {channelId, postId: rootId}), shallowEqual);

    let text = '';
    if (names.length > NAMED) {
        text = intl.formatMessage({id: 'fusion.typing.many', defaultMessage: 'Several people are typing…'});
    } else if (names.length) {
        text = intl.formatMessage(
            {id: 'fusion.typing.some', defaultMessage: '{names} {count, plural, one {is} other {are}} typing…'},
            {names: intl.formatList(names, {type: 'conjunction'}), count: names.length},
        );
    }

    return (
        <div
            className={am('typing', {on: Boolean(text)})}
            aria-live='polite'
        >
            {text && (
                <>
                    <span
                        className={am('typing-dots')}
                        aria-hidden='true'
                    >
                        <i/><i/><i/>
                    </span>
                    {text}
                </>
            )}
        </div>
    );
}
