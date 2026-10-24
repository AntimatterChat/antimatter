// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

import {am} from 'fusion/utils/class_names';

// UserTag is the small BOT or GUEST tag after someone's name.
export default function UserTag({kind}: {kind: 'bot' | 'guest'}) {
    const {formatMessage} = useIntl();
    if (kind === 'guest') {
        return <span className={am('bot-tag', 'guest-tag')}>{formatMessage({id: 'fusion.message.guest', defaultMessage: 'GUEST'})}</span>;
    }
    return <span className={am('bot-tag')}>{formatMessage({id: 'fusion.message.bot', defaultMessage: 'BOT'})}</span>;
}
