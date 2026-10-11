// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import {get, getBool} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentTimezone} from 'mattermost-redux/selectors/entities/timezone';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';
import {Preferences} from 'utils/constants';

import type {GlobalState} from 'types/store';

import {formatWalltime, offsetAt, wallDay} from './walltime';
import type {Zone} from './walltime';

// The display setting that turns walltimes off.
export const WALLTIME_PREFERENCE = 'walltime';

type Props = {
    start: number;
    end?: number;
    zone: Zone;
    explicit: boolean;
    children: React.ReactNode;
};

function readerZone(state: GlobalState): Zone {
    return getCurrentTimezone(state) || new Intl.DateTimeFormat().resolvedOptions().timeZone;
}

// WalltimeText is a time written in a message, highlighted with the reader's own time in its tooltip, when it's
// given in a time zone or the reader's isn't its author's; otherwise it's plain text.
export default function WalltimeText({start, end, zone, explicit, children}: Props) {
    const intl = useIntl();
    const reader = useSelector(readerZone);
    const hour12 = !useSelector((state: GlobalState) => getBool(state, Preferences.CATEGORY_DISPLAY_SETTINGS, Preferences.USE_MILITARY_TIME, false));
    const enabled = useSelector((state: GlobalState) => get(state, Preferences.CATEGORY_DISPLAY_SETTINGS, WALLTIME_PREFERENCE, 'true')) === 'true';
    if (!enabled || (!explicit && offsetAt(reader, start) === offsetAt(zone, start))) {
        return <>{children}</>;
    }

    // The reader's date shows when the time isn't today for them; the written one when it isn't the reader's.
    const localDay = wallDay(reader, start);
    const otherDay = localDay !== wallDay(zone, start);
    const local = formatWalltime({start, end}, reader, {locale: intl.locale, hour12, withDate: otherDay || localDay !== wallDay(reader, Date.now())});
    const written = formatWalltime({start, end}, zone, {locale: intl.locale, hour12, withDate: otherDay});
    const tip = intl.formatMessage({id: 'fusion.walltime.tip', defaultMessage: '{local} your time · {written} as written'}, {local, written});
    return (
        <time
            className={am('walltime')}
            dateTime={new Date(start).toISOString()}
            data-am-tip={tip}
        >
            <Icon
                name='clock'
                size='xs'
            />
            {children}
            <span className={am('ref-sr')}>{` (${tip})`}</span>
        </time>
    );
}
