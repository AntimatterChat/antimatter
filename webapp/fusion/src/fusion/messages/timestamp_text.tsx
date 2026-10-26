// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {getCurrentTimezone} from 'mattermost-redux/selectors/entities/timezone';

import {formatTimestamp} from 'utils/text_formatting';

// How often a relative timestamp ("in 5 minutes") is worked out again.
export const RELATIVE_TICK_MS = 30000;

type Props = {
    seconds: number;

    // t, T, d, D, f, F or R (utils/text_formatting).
    format?: string;

    // What the message's HTML had: shown if the time can't be.
    children?: React.ReactNode;
};

// TimestampText is a dynamic timestamp of a message (<t:1700000000:R>): in the reader's language and the time zone of
// their settings (not the browser's), and for a relative one, kept up to date as time passes.
export default function TimestampText({seconds, format = 'f', children}: Props) {
    const intl = useIntl();
    const zone = useSelector(getCurrentTimezone) || undefined;
    const relative = format === 'R';
    const [now, setNow] = useState(Date.now);
    useEffect(() => {
        if (!relative) {
            return undefined;
        }
        const timer = setInterval(() => setNow(Date.now()), RELATIVE_TICK_MS);
        return () => clearInterval(timer);
    }, [relative]);

    const shown = formatTimestamp(seconds, format, now, intl.locale, zone);
    return (
        <span
            className='markdown__timestamp'
            title={formatTimestamp(seconds, 'F', now, intl.locale, zone) || undefined}
        >
            {shown || children}
        </span>
    );
}
