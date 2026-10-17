// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {DateTime} from 'luxon';

import {DEFAULT_WORKDAY_START, nextWorkdayStart, workdayStart, zoneCountry} from './workday_start';

describe('fusion/composer/workday_start', () => {
    test('finds the country of a time zone', () => {
        expect(zoneCountry('Europe/Paris')).toBe('FR');
        expect(zoneCountry('Europe/Zurich')).toBe('CH');
        expect(zoneCountry('Asia/Kolkata')).toBe('IN');
        expect(zoneCountry('UTC')).toBe('');
        expect(zoneCountry('Not/A_Zone')).toBe('');
    });

    test('uses the usual start of the working day in the country', () => {
        expect(workdayStart('Europe/Berlin')).toEqual({hour: 8, minute: 0});
        expect(workdayStart('Europe/Paris')).toEqual({hour: 9, minute: 0});
        expect(workdayStart('America/New_York')).toEqual({hour: 9, minute: 0});
        expect(workdayStart('Asia/Kolkata')).toEqual({hour: 9, minute: 30});
    });

    test('falls back to 08:30', () => {
        expect(DEFAULT_WORKDAY_START).toEqual({hour: 8, minute: 30});
        expect(workdayStart('Asia/Bangkok')).toEqual(DEFAULT_WORKDAY_START);
        expect(workdayStart('UTC')).toEqual(DEFAULT_WORKDAY_START);
        expect(workdayStart('')).toEqual(DEFAULT_WORKDAY_START);
    });

    test('the next start is later the same night, or the next morning', () => {
        const night = DateTime.fromISO('2026-10-18T03:10:00', {zone: 'Asia/Bangkok'});
        expect(nextWorkdayStart(night).toISO()).toBe('2026-10-18T08:30:00.000+07:00');

        const evening = DateTime.fromISO('2026-10-17T23:00:00', {zone: 'Europe/Berlin'});
        expect(nextWorkdayStart(evening).toISO()).toBe('2026-10-18T08:00:00.000+02:00');
    });
});
