// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {DateTime} from 'luxon';
import moment from 'moment-timezone';

// When the sleep note offers to send a message "when they wake up", it picks the start of the working day in the
// reader's country, found from their time zone (moment-timezone's tz database maps zones to countries). A country
// missing from this short table of usual office hours gets 08:30, halfway between the common 08:00 and 09:00.
export const DEFAULT_WORKDAY_START = {hour: 8, minute: 30};

const WORKDAY_STARTS: Record<string, {hour: number; minute: number}> = {};
function starts(hour: number, minute: number, countries: string) {
    for (const country of countries.split(' ')) {
        WORKDAY_STARTS[country] = {hour, minute};
    }
}

// Central and northern Europe start early.
starts(8, 0, 'AT CH CZ DE DK FI HU IS NO PL SE SK');

// The "nine to five" countries.
starts(9, 0, 'AU CA CN ES FR GB HK IE IT JP KR NZ PT SG TW US');

// Indian offices usually open later.
starts(9, 30, 'IN');

// The country of an IANA time zone (its main one when several share it, e.g. Europe/Zurich: CH), or ''.
// (A zone's countries() is in moment-timezone since 0.5.25, but its typings only declare moment.tz.countries().)
type ZoneWithCountries = {countries?: () => string[]};
export function zoneCountry(zone: string): string {
    const found = moment.tz.zone(zone) as ZoneWithCountries | null;
    return found?.countries?.()[0] ?? '';
}

export function workdayStart(zone: string): {hour: number; minute: number} {
    return WORKDAY_STARTS[zoneCountry(zone)] ?? DEFAULT_WORKDAY_START;
}

// The next start of the working day after `time`, in `time`'s own zone: later today before it, tomorrow after.
export function nextWorkdayStart(time: DateTime): DateTime {
    const {hour, minute} = workdayStart(time.zoneName || '');
    let wake = time.set({hour, minute, second: 0, millisecond: 0});
    if (wake <= time) {
        wake = wake.plus({days: 1});
    }
    return wake;
}
