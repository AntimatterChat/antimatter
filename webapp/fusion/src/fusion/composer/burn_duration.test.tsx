// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {createIntl} from 'react-intl';

import {fireEvent, renderWithContext, screen, userEvent} from 'tests/react_testing_utils';

import {
    BURN_STEPS,
    BurnDurationSlider,
    burnLimitSeconds,
    burnSteps,
    clampBurnSeconds,
    defaultBurnSeconds,
    formatBurnDuration,
    readDurationSeconds,
    withReadDuration,
} from './burn_duration';

const intl = createIntl({locale: 'en', defaultLocale: 'en'});
const MIN = 60;
const HOUR = 60 * 60;
const DAY = 24 * HOUR;

describe('fusion/composer/burn_duration', () => {
    test('the steps are 1 min, 15 min, 30 min, 1 h, 4 h and 1 day', () => {
        expect(BURN_STEPS).toEqual([MIN, 15 * MIN, 30 * MIN, HOUR, 4 * HOUR, DAY]);
    });

    test('the steps above the limit are hidden', () => {
        expect(burnSteps(DAY)).toEqual(BURN_STEPS);
        expect(burnSteps(4 * HOUR)).toEqual([MIN, 15 * MIN, 30 * MIN, HOUR, 4 * HOUR]);
        expect(burnSteps(45 * MIN)).toEqual([MIN, 15 * MIN, 30 * MIN]);
        expect(burnSteps(30)).toEqual([]);
    });

    test('the limit is the administrator\'s maximum, and no longer than an unopened message lives', () => {
        expect(burnLimitSeconds({BurnOnReadMaximumDurationSeconds: '14400', BurnOnReadMaximumTimeToLiveSeconds: '604800'})).toBe(4 * HOUR);
        expect(burnLimitSeconds({BurnOnReadMaximumDurationSeconds: '86400', BurnOnReadMaximumTimeToLiveSeconds: '300'})).toBe(300);

        // A server that doesn't say: a day, as the server's default.
        expect(burnLimitSeconds({})).toBe(DAY);
        expect(burnLimitSeconds({BurnOnReadMaximumDurationSeconds: 'soon'})).toBe(DAY);
    });

    test('a duration is kept between a minute and the limit, to the second, as the server keeps it', () => {
        expect(clampBurnSeconds(90.4, DAY)).toBe(90);
        expect(clampBurnSeconds(5, DAY)).toBe(MIN);
        expect(clampBurnSeconds(30 * DAY, 4 * HOUR)).toBe(4 * HOUR);
        expect(clampBurnSeconds(Number.NaN, DAY)).toBe(MIN);
        expect(clampBurnSeconds(50, 30)).toBe(30);
    });

    test('the default is the configured duration, within the limit', () => {
        expect(defaultBurnSeconds({BurnOnReadDurationSeconds: '600'})).toBe(600);
        expect(defaultBurnSeconds({BurnOnReadDurationSeconds: '28800', BurnOnReadMaximumDurationSeconds: '3600'})).toBe(HOUR);
        expect(defaultBurnSeconds({})).toBe(600);
    });

    test('a draft carries the duration in its read_duration prop, in milliseconds', () => {
        expect(withReadDuration({spoiler: true}, 15 * MIN)).toEqual({spoiler: true, read_duration: 900000});
        expect(withReadDuration({spoiler: true, read_duration: 900000}, null)).toEqual({spoiler: true});
        expect(readDurationSeconds({read_duration: 900000})).toBe(900);
        expect(readDurationSeconds({read_duration: '3600000'})).toBe(HOUR);
        expect(readDurationSeconds({})).toBeNull();
        expect(readDurationSeconds({read_duration: -1})).toBeNull();
        expect(readDurationSeconds(undefined)).toBeNull();
    });

    test('durations read in their largest whole unit', () => {
        expect(formatBurnDuration(MIN, intl, 'short')).toBe('1 min');
        expect(formatBurnDuration(15 * MIN, intl, 'short')).toBe('15 min');
        expect(formatBurnDuration(90 * MIN, intl, 'short')).toBe('90 min');
        expect(formatBurnDuration(4 * HOUR, intl, 'short')).toBe('4 h');
        expect(formatBurnDuration(DAY, intl, 'short')).toBe('1 day');
        expect(formatBurnDuration(15 * MIN, intl)).toBe('15 minutes');
        expect(formatBurnDuration(HOUR, intl)).toBe('1 hour');
        expect(formatBurnDuration(3 * DAY, intl)).toBe('3 days');
        expect(formatBurnDuration(30, intl)).toBe('30 seconds');
    });
});

describe('fusion/composer/BurnDurationSlider', () => {
    function Harness({start, limit, onPick = jest.fn()}: {start: number; limit: number; onPick?: (s: number) => void}) {
        const [seconds, setSeconds] = useState(start);
        return (
            <>
                <BurnDurationSlider
                    seconds={seconds}
                    limit={limit}
                    onChange={(s) => {
                        setSeconds(s);
                        onPick(s);
                    }}
                />
                <output>{seconds}</output>
            </>
        );
    }
    const slider = () => screen.getByRole('slider', {name: 'Deleted after reading'});
    const picked = () => Number(screen.getByRole('status').textContent);

    test('steps through the durations the limit allows, then Custom', () => {
        renderWithContext(
            <Harness
                start={15 * MIN}
                limit={HOUR}
            />,
        );
        expect(slider()).toHaveAttribute('max', '4');
        expect(slider()).toHaveValue('1');
        expect(slider()).toHaveAttribute('aria-valuetext', '15 minutes');

        // The labels: the allowed steps and Custom; 4 h and 1 day are over the limit.
        expect(screen.getByText('1 h')).toBeInTheDocument();
        expect(screen.queryByText('4 h')).not.toBeInTheDocument();
        expect(screen.queryByText('1 day')).not.toBeInTheDocument();
        expect(screen.getByText('Custom')).toBeInTheDocument();

        fireEvent.change(slider(), {target: {value: '3'}});
        expect(picked()).toBe(HOUR);
        expect(slider()).toHaveAttribute('aria-valuetext', '1 hour');
        expect(screen.queryByRole('spinbutton')).not.toBeInTheDocument();
    });

    test('a step\'s label jumps to it', async () => {
        renderWithContext(
            <Harness
                start={MIN}
                limit={DAY}
            />,
        );
        await userEvent.click(screen.getByText('1 day'));
        expect(picked()).toBe(DAY);
        expect(slider()).toHaveValue('5');
    });

    test('Custom takes any duration up to the limit, and keeps a longer one at it', async () => {
        const onPick = jest.fn();
        renderWithContext(
            <Harness
                start={30 * MIN}
                limit={4 * HOUR}
                onPick={onPick}
            />,
        );
        fireEvent.change(slider(), {target: {value: '5'}});
        const count = screen.getByRole('spinbutton', {name: 'Custom duration'});
        const unit = screen.getByRole('combobox', {name: 'Unit'});

        // It starts from the duration picked, and lists no unit longer than the limit.
        expect(count).toHaveValue(30);
        expect(unit).toHaveValue(String(MIN));
        expect(screen.queryByRole('option', {name: 'days'})).not.toBeInTheDocument();
        expect(screen.getByText('Up to 4 hours')).toBeInTheDocument();

        await userEvent.clear(count);
        await userEvent.type(count, '45');
        expect(picked()).toBe(45 * MIN);

        // Still Custom, even on a duration that is a step.
        await userEvent.clear(count);
        await userEvent.type(count, '15');
        expect(picked()).toBe(15 * MIN);
        expect(slider()).toHaveValue('5');

        await userEvent.selectOptions(unit, String(HOUR));
        expect(picked()).toBe(4 * HOUR);
        expect(count).toHaveAttribute('aria-invalid', 'true');
        expect(onPick).not.toHaveBeenCalledWith(15 * HOUR);
    });

    test('a duration that isn\'t a step starts on Custom', () => {
        renderWithContext(
            <Harness
                start={10 * MIN}
                limit={DAY}
            />,
        );
        expect(slider()).toHaveValue('6');
        expect(slider()).toHaveAttribute('aria-valuetext', 'Custom: 10 minutes');
        expect(screen.getByRole('spinbutton', {name: 'Custom duration'})).toHaveValue(10);
    });
});
