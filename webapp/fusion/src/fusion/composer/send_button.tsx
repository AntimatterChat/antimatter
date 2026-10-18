// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

type Props = {
    disabled: boolean;

    // What it says, when it doesn't send a plain message (a composer mode's, e.g. "Post poll").
    label?: string;

    // With scheduled messages on, a chevron next to the button opens the schedule picker ("Send later").
    laterRef?: React.Ref<HTMLButtonElement>;
    laterOpen?: boolean;
    onLater?: () => void;
};

// SendButton submits the composer's form; its chevron schedules the message instead.
export default function SendButton({disabled, label, laterRef, laterOpen = false, onLater}: Props) {
    const {formatMessage} = useIntl();
    return (
        <>
            <button
                className={am('icon-btn', 'send-btn', {split: Boolean(onLater)})}
                aria-label={label || formatMessage({id: 'fusion.composer.send', defaultMessage: 'Send'})}
                title={label}
                disabled={disabled}
            >
                <Icon
                    name='send'
                    size='sm'
                />
            </button>
            {onLater && (
                <button
                    ref={laterRef}
                    type='button'
                    className={am('icon-btn', 'send-later', {on: laterOpen})}
                    title={formatMessage({id: 'fusion.composer.sendLater', defaultMessage: 'Send later'})}
                    aria-label={formatMessage({id: 'fusion.composer.sendLater', defaultMessage: 'Send later'})}
                    aria-haspopup='menu'
                    aria-expanded={laterOpen}
                    disabled={disabled}
                    onClick={onLater}
                >
                    <Icon
                        name='chev'
                        size='xs'
                    />
                </button>
            )}
        </>
    );
}
