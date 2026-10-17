// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useId, useState} from 'react';
import {useIntl} from 'react-intl';
import type {IntlShape} from 'react-intl';

import type {ClientConfig} from '@mattermost/types/config';

import {am} from 'fusion/utils/class_names';

// How long a burn-on-read message lives once read, which its sender picks on a slider of steps (or a custom duration),
// within the administrator's maximum. The message carries it in its read_duration prop, in milliseconds; the server
// checks and clamps it (model.BurnOnReadDurationMillis), and uses the configured duration when there's none.

// The slider's steps, in seconds: 1 min, 15 min, 30 min, 1 h, 4 h, 1 day.
export const BURN_STEPS = [60, 15 * 60, 30 * 60, 60 * 60, 4 * 60 * 60, 24 * 60 * 60];

// The shortest duration, as on the server.
export const BURN_MIN_SECONDS = 60;

const DAY = 24 * 60 * 60;
const HOUR = 60 * 60;

const positive = (value: string | undefined) => {
    const n = parseInt(value || '', 10);
    return Number.isFinite(n) && n > 0 ? n : 0;
};

// burnLimitSeconds is the longest a sender may pick: the administrator's maximum, and never longer than an unopened
// message lives (the maximum time to live), past which a longer duration would change nothing.
export function burnLimitSeconds(config: Partial<ClientConfig>): number {
    const max = positive(config.BurnOnReadMaximumDurationSeconds) || DAY;
    const ttl = positive(config.BurnOnReadMaximumTimeToLiveSeconds);
    return ttl ? Math.min(max, ttl) : max;
}

// clampBurnSeconds keeps a duration between a minute and the limit, to the second, as the server does.
export function clampBurnSeconds(seconds: number, limit: number): number {
    const lowest = Math.min(BURN_MIN_SECONDS, limit);
    if (!Number.isFinite(seconds)) {
        return lowest;
    }
    return Math.min(limit, Math.max(lowest, Math.round(seconds)));
}

// burnSteps are the slider's steps the limit allows: the ones above it are hidden.
export function burnSteps(limit: number): number[] {
    return BURN_STEPS.filter((s) => s <= limit);
}

// defaultBurnSeconds is the duration of a message whose sender picks none: the configured one, within the limit.
export function defaultBurnSeconds(config: Partial<ClientConfig>, limit = burnLimitSeconds(config)): number {
    return clampBurnSeconds(positive(config.BurnOnReadDurationSeconds) || 600, limit);
}

// readDurationSeconds is the duration a draft or a message carries (its read_duration prop, in milliseconds), if any.
export function readDurationSeconds(props: Record<string, unknown> | undefined): number | null {
    const value = props?.read_duration;
    const ms = typeof value === 'number' ? value : parseInt(String(value ?? ''), 10);
    return Number.isFinite(ms) && ms > 0 ? Math.round(ms / 1000) : null;
}

// withReadDuration sets a draft's read_duration prop (null removes it).
export function withReadDuration(props: Record<string, unknown> | undefined, seconds: number | null): Record<string, unknown> {
    const next = {...props};
    if (seconds === null) {
        delete next.read_duration;
    } else {
        next.read_duration = seconds * 1000;
    }
    return next;
}

// formatBurnDuration says a duration in the largest whole unit: "15 min" / "15 minutes", "4 h" / "4 hours", "1 day".
export function formatBurnDuration(seconds: number, intl: IntlShape, style: 'short' | 'long' = 'long'): string {
    const {formatMessage} = intl;
    if (seconds % DAY === 0) {
        return formatMessage({id: 'fusion.burn.days', defaultMessage: '{count, plural, one {# day} other {# days}}'}, {count: seconds / DAY});
    }
    if (seconds >= HOUR && seconds % HOUR === 0) {
        return style === 'short' ? formatMessage({id: 'fusion.burn.hoursShort', defaultMessage: '{count} h'}, {count: seconds / HOUR}) : formatMessage({id: 'fusion.burn.hours', defaultMessage: '{count, plural, one {# hour} other {# hours}}'}, {count: seconds / HOUR});
    }
    if (seconds >= 60) {
        const minutes = Math.round(seconds / 60);
        return style === 'short' ? formatMessage({id: 'fusion.burn.minutesShort', defaultMessage: '{count} min'}, {count: minutes}) : formatMessage({id: 'fusion.burn.minutes', defaultMessage: '{count, plural, one {# minute} other {# minutes}}'}, {count: minutes});
    }
    return formatMessage({id: 'fusion.burn.seconds', defaultMessage: '{count, plural, one {# second} other {# seconds}}'}, {count: seconds});
}

// The custom duration's units, in seconds.
const UNITS = [60, HOUR, DAY] as const;
type Unit = typeof UNITS[number];

// splitDuration gives a duration as a count of its largest whole unit.
function splitDuration(seconds: number): {count: number; unit: Unit} {
    if (seconds % DAY === 0) {
        return {count: seconds / DAY, unit: DAY};
    }
    if (seconds % HOUR === 0) {
        return {count: seconds / HOUR, unit: HOUR};
    }
    return {count: Math.max(1, Math.round(seconds / 60)), unit: 60};
}

type Props = {

    // The duration picked, in seconds.
    seconds: number;

    // The longest that may be picked (burnLimitSeconds).
    limit: number;
    onChange: (seconds: number) => void;
};

// BurnDurationSlider picks how long a burn-on-read message lives once read: a slider of steps (those the limit allows),
// its last position "Custom", where a count and a unit give any duration up to the limit. Its labels under it are
// buttons too, for a pointer to jump to a step.
export function BurnDurationSlider({seconds, limit, onChange}: Props) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const steps = burnSteps(limit);
    const [custom, setCustom] = useState(() => !steps.includes(seconds));
    const [draft, setDraft] = useState(() => splitDuration(seconds));
    const id = useId();
    const customIndex = steps.length;
    const index = custom ? customIndex : Math.max(0, steps.indexOf(seconds));
    const customLabel = formatMessage({id: 'fusion.burn.custom', defaultMessage: 'Custom'});
    const labelOf = (i: number) => (i === customIndex ? customLabel : formatBurnDuration(steps[i], intl, 'short'));

    const pick = (i: number) => {
        if (i >= customIndex) {
            setCustom(true);
            setDraft(splitDuration(seconds));
            return;
        }
        setCustom(false);
        onChange(steps[i]);
    };

    const setCustomDuration = (count: number, unit: Unit) => {
        setDraft({count, unit});
        if (count > 0) {
            onChange(clampBurnSeconds(count * unit, limit));
        }
    };

    const units = UNITS.filter((u) => u <= limit);
    const over = custom && draft.count * draft.unit > limit;
    return (
        <div className={am('burn-slider')}>
            <input
                type='range'
                min={0}
                max={customIndex}
                step={1}
                value={index}
                aria-label={formatMessage({id: 'fusion.burn.durationLabel', defaultMessage: 'Deleted after reading'})}
                aria-valuetext={custom ? `${customLabel}: ${formatBurnDuration(seconds, intl)}` : formatBurnDuration(steps[index], intl)}
                style={{'--am-range': `${(index / Math.max(1, customIndex)) * 100}%`} as React.CSSProperties}
                onChange={(e) => pick(parseInt(e.target.value, 10))}
            />
            <div
                className={am('burn-steps')}
                aria-hidden='true'
            >
                {[...steps, null].map((step, i) => (
                    <button
                        key={step ?? 'custom'}
                        type='button'
                        tabIndex={-1}
                        className={am({on: i === index})}
                        onClick={() => pick(i)}
                    >
                        {labelOf(i)}
                    </button>
                ))}
            </div>
            {custom && (
                <div className={am('burn-custom')}>
                    <label htmlFor={`${id}-count`}>{formatMessage({id: 'fusion.burn.customLabel', defaultMessage: 'Custom duration'})}</label>
                    <input
                        id={`${id}-count`}
                        type='number'
                        inputMode='numeric'
                        min={1}
                        value={draft.count || ''}
                        aria-invalid={over}
                        aria-describedby={`${id}-limit`}
                        onChange={(e) => setCustomDuration(parseInt(e.target.value, 10) || 0, draft.unit)}
                    />
                    <select
                        value={draft.unit}
                        aria-label={formatMessage({id: 'fusion.burn.customUnit', defaultMessage: 'Unit'})}
                        onChange={(e) => setCustomDuration(draft.count, parseInt(e.target.value, 10) as Unit)}
                    >
                        {units.map((u) => (
                            <option
                                key={u}
                                value={u}
                            >
                                {u === 60 && formatMessage({id: 'fusion.burn.unitMinutes', defaultMessage: 'minutes'})}
                                {u === HOUR && formatMessage({id: 'fusion.burn.unitHours', defaultMessage: 'hours'})}
                                {u === DAY && formatMessage({id: 'fusion.burn.unitDays', defaultMessage: 'days'})}
                            </option>
                        ))}
                    </select>
                    <small
                        id={`${id}-limit`}
                        className={am({over})}
                    >
                        {formatMessage({id: 'fusion.burn.upTo', defaultMessage: 'Up to {max}'}, {max: formatBurnDuration(limit, intl)})}
                    </small>
                </div>
            )}
        </div>
    );
}
