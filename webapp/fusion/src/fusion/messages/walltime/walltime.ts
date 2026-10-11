// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Walltime: the times written in messages ("3pm", "tomorrow at 15:30 CET", "10-11am PT"), read in the time zone they
// were written in (the one given, or else the author's) on the day they were posted, so each reader can see them in
// their own time zone, as the community Walltime plugin did
// (github.com/mattermost-community/mattermost-plugin-walltime, Apache License 2.0).

// A time zone: an IANA name, or a fixed offset from UTC as "+05:30".
export type Zone = string;

export type Walltime = {

    // Where the time is in the text, and its text.
    index: number;
    text: string;

    // When it is (and when it ends, for a range), in milliseconds.
    start: number;
    end?: number;

    // The time zone it's written in, and whether the text gives it.
    zone: Zone;
    explicit: boolean;
};

// Time zone abbreviations, as people mean them: "EST" in July is New York time, not UTC-5.
const ABBREVIATIONS: Record<string, Zone> = {
    UTC: 'UTC',
    GMT: 'UTC',
    CET: 'Europe/Paris',
    CEST: 'Europe/Paris',
    EET: 'Europe/Athens',
    EEST: 'Europe/Athens',
    WET: 'Europe/Lisbon',
    WEST: 'Europe/Lisbon',
    BST: 'Europe/London',
    MSK: 'Europe/Moscow',
    IST: 'Asia/Kolkata',
    SGT: 'Asia/Singapore',
    HKT: 'Asia/Hong_Kong',
    JST: 'Asia/Tokyo',
    KST: 'Asia/Seoul',
    ET: 'America/New_York',
    EST: 'America/New_York',
    EDT: 'America/New_York',
    CT: 'America/Chicago',
    CST: 'America/Chicago',
    CDT: 'America/Chicago',
    MT: 'America/Denver',
    MST: 'America/Denver',
    MDT: 'America/Denver',
    PT: 'America/Los_Angeles',
    PST: 'America/Los_Angeles',
    PDT: 'America/Los_Angeles',
    AKST: 'America/Anchorage',
    AKDT: 'America/Anchorage',
    HST: 'Pacific/Honolulu',
    BRT: 'America/Sao_Paulo',
    AEST: 'Australia/Sydney',
    AEDT: 'Australia/Sydney',
    ACST: 'Australia/Adelaide',
    ACDT: 'Australia/Adelaide',
    AWST: 'Australia/Perth',
    NZST: 'Pacific/Auckland',
    NZDT: 'Pacific/Auckland',
};

const WEEKDAYS = ['sunday', 'monday', 'tuesday', 'wednesday', 'thursday', 'friday', 'saturday'];

// The parts of the pattern. It's case-sensitive, so that zone abbreviations are capitals only ("est" and "art" are
// words); the rest takes a capital too.
const cap = (word: string) => `[${word[0].toUpperCase()}${word[0]}]${word.slice(1)}`;
const DAY = `(?:${cap('today')}|${cap('tonight')}|${cap('tomorrow')}|(?:${cap('next')}\\s)?(?:${WEEKDAYS.map(cap).join('|')}))`;
const TIME12 = '\\d{1,2}(?::[0-5]\\d)?\\s?(?:[aApP][mM](?!\\w)|[aApP]\\.[mM]\\.)';
const TIME24 = '(?:[01]?\\d|2[0-3])[:h][0-5]\\d(?!:\\d|\\w)';
const TIME = `(?:${TIME12}|${TIME24}|${cap('noon')}(?!\\w)|${cap('midnight')}(?!\\w))`;
const BARE = '\\d{1,2}(?::[0-5]\\d)?';
const SEP = '(?:\\s?[-–—]\\s?|\\s(?:to|until|till)\\s)';
const ZONE = `(?:\\s?\\(?(?:(?:UTC|GMT|utc|gmt)\\s?[+\\-−]\\s?\\d{1,2}(?::?\\d{2})?|[A-Z][a-z]+\\/[A-Za-z_]+(?:\\/[A-Za-z_]+)?|${Object.keys(ABBREVIATIONS).join('|')}|utc|gmt)\\)?(?!\\w))`;

// 1: the day before ("tomorrow at"), 2: a range's start, 3: the time, 4: the zone, 5: the day after ("on Friday").
const PATTERN = new RegExp(
    `(?<![\\w:.\\/$+\\-])(?:(${DAY}),?\\s(?:at\\s|@\\s?)?)?(?:(${TIME}|${BARE})${SEP})?(${TIME})(${ZONE})?(?:,?\\s(?:on\\s)?(${DAY})(?!\\w))?`,
    'g',
);

const zoneCache = new Map<string, boolean>();

export function isValidZone(zone: string): boolean {
    if ((/^[+-]\d{2}:\d{2}$/).test(zone)) {
        return true;
    }
    let valid = zoneCache.get(zone);
    if (valid === undefined) {
        try {
            new Intl.DateTimeFormat('en-US', {timeZone: zone}); // eslint-disable-line no-new
            valid = true;
        } catch {
            valid = false;
        }
        zoneCache.set(zone, valid);
    }
    return valid;
}

// parseZone reads a zone as written after a time; undefined when it isn't one.
export function parseZone(written: string): Zone | undefined {
    const text = written.trim().replace(/^\(|\)$/g, '');
    const offset = (/^(?:UTC|GMT|utc|gmt)\s?([+\-−])\s?(\d{1,2})(?::?(\d{2}))?$/).exec(text);
    if (offset) {
        const hours = Number(offset[2]);
        const minutes = Number(offset[3] || 0);
        if (hours > 14 || minutes > 59) {
            return undefined;
        }
        return `${offset[1] === '+' ? '+' : '-'}${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}`;
    }
    const zone = ABBREVIATIONS[text] || ((/^(?:utc|gmt)$/).test(text) ? 'UTC' : text);
    return zone.includes('/') || zone === 'UTC' ? (isValidZone(zone) && zone) || undefined : undefined;
}

const formatters = new Map<string, Intl.DateTimeFormat>();

// offsetAt is a zone's offset from UTC at a time, in minutes.
export function offsetAt(zone: Zone, time: number): number {
    const fixed = (/^([+-])(\d{2}):(\d{2})$/).exec(zone);
    if (fixed) {
        return (fixed[1] === '-' ? -1 : 1) * ((Number(fixed[2]) * 60) + Number(fixed[3]));
    }
    let format = formatters.get(zone);
    if (!format) {
        format = new Intl.DateTimeFormat('en-US', {timeZone: zone, hourCycle: 'h23', year: 'numeric', month: 'numeric', day: 'numeric', hour: 'numeric', minute: 'numeric'});
        formatters.set(zone, format);
    }
    const parts: Record<string, number> = {};
    for (const part of format.formatToParts(new Date(time))) {
        parts[part.type] = Number(part.value);
    }
    const local = Date.UTC(parts.year, parts.month - 1, parts.day, parts.hour % 24, parts.minute);
    return Math.round((local - (time - (time % 60000))) / 60000);
}

// wallDate gives the date of a time in a zone, and its weekday.
function wallDate(zone: Zone, time: number) {
    const shifted = new Date(time + (offsetAt(zone, time) * 60000));
    return {year: shifted.getUTCFullYear(), month: shifted.getUTCMonth(), day: shifted.getUTCDate(), weekday: shifted.getUTCDay()};
}

// zonedTime is the moment a wall clock in a zone shows a date and time (days and hours past their end roll over).
export function zonedTime(zone: Zone, year: number, month: number, day: number, hour: number, minute: number): number {
    const wall = Date.UTC(year, month, day, hour, minute);
    let time = wall - (offsetAt(zone, wall) * 60000);

    // Near a change of offset, the offset at the guess may not be the one at the answer.
    const again = wall - (offsetAt(zone, time) * 60000);
    if (again !== time) {
        time = again;
    }
    return time;
}

type Clock = {hour: number; minute: number; meridiem?: 'a' | 'p'};

function readClock(text: string): Clock | undefined {
    const word = text.toLowerCase();
    if (word === 'noon') {
        return {hour: 12, minute: 0};
    }
    if (word === 'midnight') {
        return {hour: 24, minute: 0};
    }
    const time = (/^(\d{1,2})(?:[:h](\d{2}))?\s?(?:([ap])\.?m\.?)?$/i).exec(text.trim());
    if (!time) {
        return undefined;
    }
    const hour = Number(time[1]);
    const minute = Number(time[2] || 0);
    const meridiem = time[3]?.toLowerCase() as Clock['meridiem'];
    if (meridiem ? hour < 1 || hour > 12 : hour > 23) {
        return undefined;
    }
    return {hour, minute, meridiem};
}

function to24(clock: Clock): number {
    if (!clock.meridiem) {
        return clock.hour;
    }
    return (clock.hour % 12) + (clock.meridiem === 'p' ? 12 : 0);
}

// dayOffset is how many days after the posting a day word means.
function dayOffset(word: string | undefined, postedWeekday: number): number {
    if (!word) {
        return 0;
    }
    const lower = word.toLowerCase();
    if (lower === 'today' || lower === 'tonight') {
        return 0;
    }
    if (lower === 'tomorrow') {
        return 1;
    }
    const next = lower.startsWith('next');
    const weekday = WEEKDAYS.indexOf(lower.replace(/^next\s/, ''));
    const ahead = ((weekday - postedWeekday) + 7) % 7;
    return next && ahead === 0 ? 7 : ahead;
}

// findWalltimes finds the times of a text, read in the zone they give or else in authorZone, on the day of postedAt.
// Times without a zone, when the author's isn't known, are left out.
export function findWalltimes(text: string, authorZone: Zone | undefined, postedAt: number): Walltime[] {
    const found: Walltime[] = [];
    if (!(/\d|noon|midnight/i).test(text)) {
        return found;
    }
    PATTERN.lastIndex = 0;
    for (let m = PATTERN.exec(text); m; m = PATTERN.exec(text)) {
        const [whole, dayBefore, rangeStart, time, zoneText, dayAfter] = m;
        let matched = whole;
        let day = dayBefore || dayAfter;
        let zone = zoneText ? parseZone(zoneText) : undefined;
        if (zoneText && !zone) {
            // Not a zone after all ("3pm Ops/Infra"): the time ends before it.
            matched = whole.slice(0, whole.indexOf(zoneText));
            day = dayBefore;
        }
        const explicit = Boolean(zone);
        zone = zone || authorZone;
        const clock = readClock(time);
        if (!zone || !isValidZone(zone) || !clock) {
            continue;
        }

        const tonight = day?.toLowerCase() === 'tonight';
        let hour = to24(clock);
        let minute = clock.minute;
        let endHour: number | undefined;
        if (rangeStart) {
            const first = readClock(rangeStart);
            if (!first) {
                continue;
            }

            // "10-11am", "11-1pm": the start takes the end's half of the day, or the half before.
            if (!first.meridiem && clock.meridiem && first.hour <= 12) {
                const same = to24({...first, meridiem: clock.meridiem});
                first.meridiem = (same * 60) + first.minute > (hour * 60) + minute ? 'a' : clock.meridiem;
            }
            endHour = hour;
            hour = to24(first);
            minute = first.minute;
            if (tonight && !clock.meridiem && endHour < 12) {
                endHour += 12;
            }
        }
        if (tonight && !clock.meridiem && hour < 12 && !(rangeStart && (/[ap]\.?m/i).test(rangeStart))) {
            hour += 12;
        }

        const posted = wallDate(zone, postedAt);
        const date = [posted.year, posted.month, posted.day + dayOffset(day, posted.weekday)] as const;
        const start = zonedTime(zone, ...date, hour, minute);
        const walltime: Walltime = {index: m.index, text: matched, start, zone, explicit};
        if (endHour !== undefined) {
            walltime.end = zonedTime(zone, ...date, endHour, clock.minute);
            if (walltime.end <= start) {
                walltime.end += 24 * 3600 * 1000;
            }
        }
        found.push(walltime);
    }
    return found;
}

// wallDay names the date of a time in a zone, to compare dates.
export function wallDay(zone: Zone, time: number): string {
    const date = wallDate(zone, time);
    return `${date.year}-${date.month + 1}-${date.day}`;
}

// formatWalltime shows a time (or a range) in a zone, with the zone's name, and its date when withDate.
export function formatWalltime(walltime: Pick<Walltime, 'start' | 'end'>, zone: Zone, {locale, hour12, withDate}: {locale?: string; hour12?: boolean; withDate?: boolean}): string {
    const options: Intl.DateTimeFormatOptions = {hour: 'numeric', minute: '2-digit', hour12, ...(withDate ? {weekday: 'short', month: 'short', day: 'numeric'} : {})};
    const show = (format: Intl.DateTimeFormat, shift: number) => (walltime.end ? format.formatRange(walltime.start + shift, walltime.end + shift) : format.format(walltime.start + shift));
    try {
        return show(new Intl.DateTimeFormat(locale, {...options, timeZone: zone, timeZoneName: 'short'}), 0);
    } catch {
        // A fixed offset, where Intl doesn't take one: UTC moved by it, named as the offset.
        return `${show(new Intl.DateTimeFormat(locale, {...options, timeZone: 'UTC'}), offsetAt(zone, walltime.start) * 60000)} UTC${zone}`;
    }
}
