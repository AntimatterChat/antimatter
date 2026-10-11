// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {findWalltimes, formatWalltime, offsetAt, parseZone, wallDay, zonedTime} from './walltime';

// Friday, October 9 2026, 10:00 in New York.
const POSTED = Date.UTC(2026, 9, 9, 14, 0);
const NY = 'America/New_York';
const at = (iso: string) => Date.parse(iso);
const plain = (text: string) => text.replace(/\s/g, ' ');

describe('walltime', () => {
    test.each([
        ['Let\'s meet at 3pm', '3pm', '2026-10-09T19:00:00Z'],
        ['call at 3:30 p.m. EST then', '3:30 p.m. EST', '2026-10-09T19:30:00Z'],
        ['standup 9:15 sharp', '9:15', '2026-10-09T13:15:00Z'],
        ['tomorrow at 15:30 CET works', 'tomorrow at 15:30 CET', '2026-10-10T13:30:00Z'],
        ['Monday 9:00 is fine', 'Monday 9:00', '2026-10-12T13:00:00Z'],
        ['15:00 on Friday', '15:00 on Friday', '2026-10-09T19:00:00Z'],
        ['next Friday at noon', 'next Friday at noon', '2026-10-16T16:00:00Z'],
        ['tonight at 8:00', 'tonight at 8:00', '2026-10-10T00:00:00Z'],
        ['À 15h30 UTC+2 alors', '15h30 UTC+2', '2026-10-09T13:30:00Z'],
        ['window opens 22:00 UTC.', '22:00 UTC', '2026-10-09T22:00:00Z'],
        ['until midnight UTC', 'midnight UTC', '2026-10-10T00:00:00Z'],
        ['at 9am (Europe/Berlin)', '9am (Europe/Berlin)', '2026-10-09T07:00:00Z'],
        ['ship at 4pm Ops/Infra', '4pm', '2026-10-09T20:00:00Z'],
    ])('reads %j', (text, written, iso) => {
        const [found] = findWalltimes(text, NY, POSTED);
        expect(found).toMatchObject({text: written, start: at(iso)});
        expect(text.slice(found.index, found.index + found.text.length)).toBe(written);
    });

    test('tells the zones given from the author\'s', () => {
        const [given, implied] = findWalltimes('10:00 PT, so 13:00 for me', NY, POSTED);
        expect(given).toMatchObject({zone: 'America/Los_Angeles', explicit: true});
        expect(implied).toMatchObject({zone: NY, explicit: false});
    });

    test.each([
        ['10-11am PT', '2026-10-09T17:00:00Z', '2026-10-09T18:00:00Z'],
        ['11-1pm', '2026-10-09T15:00:00Z', '2026-10-09T17:00:00Z'],
        ['9:30 to 11:00', '2026-10-09T13:30:00Z', '2026-10-09T15:00:00Z'],
        ['23:00–01:00 UTC', '2026-10-09T23:00:00Z', '2026-10-10T01:00:00Z'],
    ])('reads the range %j', (text, start, end) => {
        expect(findWalltimes(text, NY, POSTED)).toEqual([expect.objectContaining({text, start: at(start), end: at(end)})]);
    });

    test.each([
        'version 1.2:30',
        'a 1:2 ratio',
        'on 2026-10-11',
        'logged at 12:30:45',
        'took 3h',
        'pages 3-4',
        'the 25:00 mark',
        'costs $3pm',
        'they sat at noonish',
        'Mr. 3pmx',
    ])('finds no time in %j', (text) => {
        expect(findWalltimes(text, NY, POSTED)).toEqual([]);
    });

    test('needs a zone: the one given, or the author\'s', () => {
        expect(findWalltimes('at 3pm', undefined, POSTED)).toEqual([]);
        expect(findWalltimes('at 3pm UTC', undefined, POSTED)).toEqual([expect.objectContaining({start: at('2026-10-09T15:00:00Z'), zone: 'UTC'})]);
    });

    test('reads the day in the zone written, across a change of offset', () => {
        // Friday October 23; Paris leaves summer time on Sunday the 25th.
        const posted = Date.UTC(2026, 9, 23, 9, 0);
        expect(findWalltimes('Monday 9:00 CET', NY, posted)[0].start).toBe(at('2026-10-26T08:00:00Z'));
        expect(findWalltimes('Friday 9:00 CET', NY, posted)[0].start).toBe(at('2026-10-23T07:00:00Z'));
    });

    test('reads zones', () => {
        expect(parseZone(' UTC-05:30')).toBe('-05:30');
        expect(parseZone('GMT+1')).toBe('+01:00');
        expect(parseZone('(PST)')).toBe('America/Los_Angeles');
        expect(parseZone('utc')).toBe('UTC');
        expect(parseZone('Mars/Olympus')).toBeUndefined();
        expect(parseZone('UTC+25')).toBeUndefined();
    });

    test('knows offsets and wall clocks', () => {
        expect(offsetAt(NY, at('2026-07-01T12:00:00Z'))).toBe(-240);
        expect(offsetAt(NY, at('2026-12-01T12:00:00Z'))).toBe(-300);
        expect(offsetAt('+05:30', 0)).toBe(330);
        expect(zonedTime('Asia/Kolkata', 2026, 9, 9, 9, 0)).toBe(at('2026-10-09T03:30:00Z'));
        expect(wallDay('Pacific/Auckland', at('2026-10-09T14:00:00Z'))).toBe('2026-10-10');
    });

    test('shows times in a zone', () => {
        expect(plain(formatWalltime({start: at('2026-10-09T19:00:00Z')}, NY, {locale: 'en-US', hour12: true}))).toBe('3:00 PM EDT');
        expect(plain(formatWalltime({start: at('2026-10-09T19:00:00Z')}, 'Europe/Paris', {locale: 'en-GB', hour12: false, withDate: true}))).toMatch(/^Fri,? 9 Oct,? 21:00 CEST$/);
        expect(plain(formatWalltime({start: at('2026-10-09T17:00:00Z'), end: at('2026-10-09T18:00:00Z')}, 'America/Los_Angeles', {locale: 'en-US', hour12: true}))).toBe('10:00 – 11:00 AM PDT');
        expect(plain(formatWalltime({start: at('2026-10-09T13:30:00Z')}, '+02:00', {locale: 'en-US', hour12: false}))).toMatch(/^15:30 (UTC\+02:00|GMT\+2)$/);
    });
});
