// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {DateTime} from 'luxon';
import React, {useState} from 'react';
import {useIntl} from 'react-intl';

import type {FocusRequest} from '@mattermost/types/focus';

import {Client4} from 'mattermost-redux/client';

import {Dialog} from 'fusion/components/layer';
import {useMyZone} from 'fusion/composer/schedule';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

// The reason becomes the custom status: its length.
export const FOCUS_REASON_MAX = 100;

// The longest focus the server takes, and the shortest.
const MAX_FOCUS = 7 * 24 * 36e5;
const MIN_FOCUS = 6e4;

type Choice = {key: string; duration?: string; minutes?: number; label: {id: string; defaultMessage: string; description: string}};

export const FOCUS_DURATIONS: Choice[] = [
    {key: '15m', duration: '15m', minutes: 15, label: {id: 'fusion.focus.for15m', defaultMessage: '15 min', description: 'Focus mode dialog: a duration chip, focus for 15 minutes.'}},
    {key: '30m', duration: '30m', minutes: 30, label: {id: 'fusion.focus.for30m', defaultMessage: '30 min', description: 'Focus mode dialog: a duration chip, focus for 30 minutes.'}},
    {key: '1h', duration: '1h', minutes: 60, label: {id: 'fusion.focus.for1h', defaultMessage: '1 h', description: 'Focus mode dialog: a duration chip, focus for 1 hour.'}},
    {key: '2h', duration: '2h', minutes: 120, label: {id: 'fusion.focus.for2h', defaultMessage: '2 h', description: 'Focus mode dialog: a duration chip, focus for 2 hours.'}},
    {key: '3h', duration: '3h', minutes: 180, label: {id: 'fusion.focus.for3h', defaultMessage: '3 h', description: 'Focus mode dialog: a duration chip, focus for 3 hours.'}},
    {key: '4h', duration: '4h', minutes: 240, label: {id: 'fusion.focus.for4h', defaultMessage: '4 h', description: 'Focus mode dialog: a duration chip, focus for 4 hours.'}},
    {key: '5h', duration: '5h', minutes: 300, label: {id: 'fusion.focus.for5h', defaultMessage: '5 h', description: 'Focus mode dialog: a duration chip, focus for 5 hours.'}},
    {key: '6h', duration: '6h', minutes: 360, label: {id: 'fusion.focus.for6h', defaultMessage: '6 h', description: 'Focus mode dialog: a duration chip, focus for 6 hours.'}},
    {key: 'tomorrow', duration: 'tomorrow', label: {id: 'fusion.focus.tomorrow', defaultMessage: 'Until tomorrow', description: 'Focus mode dialog: a duration chip, focus until 9:00 the next morning in your timezone.'}},
    {key: 'custom', label: {id: 'fusion.focus.custom', defaultMessage: 'Custom', description: 'Focus mode dialog: a duration chip that shows a date and time field to choose when the focus ends.'}},
];

// tomorrowAt9 is 09:00 tomorrow in your timezone, as the server works it out for "tomorrow".
export function tomorrowAt9(zone: string, now = Date.now()): number {
    return DateTime.fromMillis(now, {zone}).plus({days: 1}).set({hour: 9, minute: 0, second: 0, millisecond: 0}).toMillis();
}

// FocusDialog starts focus mode: how long, from the chips (or a custom end), and why, which becomes your custom
// status with 🎯 while you focus.
export default function FocusDialog({onClose}: {onClose: () => void}) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const toast = useToast();
    const zone = useMyZone();
    const [choice, setChoice] = useState('1h');
    const [custom, setCustom] = useState(() => DateTime.now().setZone(zone).plus({hours: 2}).startOf('hour').toFormat("yyyy-MM-dd'T'HH:mm"));
    const [reason, setReason] = useState('');
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');

    const picked = FOCUS_DURATIONS.find((d) => d.key === choice) || FOCUS_DURATIONS[2];
    const customAt = DateTime.fromISO(custom, {zone});
    const customValid = customAt.isValid && customAt.toMillis() - Date.now() >= MIN_FOCUS && customAt.toMillis() - Date.now() <= MAX_FOCUS;
    let end = 0;
    if (picked.minutes) {
        end = Date.now() + (picked.minutes * 6e4);
    } else if (picked.key === 'tomorrow') {
        end = tomorrowAt9(zone);
    } else if (customValid) {
        end = customAt.toMillis();
    }
    const sameDay = end && DateTime.fromMillis(end, {zone}).hasSame(DateTime.now().setZone(zone), 'day');
    const endLabel = end ? intl.formatDate(end, sameDay ? {hour: 'numeric', minute: '2-digit', timeZone: zone} : {weekday: 'short', day: 'numeric', month: 'short', hour: 'numeric', minute: '2-digit', timeZone: zone}) : '';

    const title = formatMessage({id: 'fusion.focus.title', defaultMessage: 'Focus mode', description: 'Title of the dialog that starts focus mode (Do not disturb for a while, with a reason).'});

    const submit = async () => {
        if (busy || !end) {
            return;
        }
        const request: FocusRequest = picked.duration ? {duration: picked.duration, reason: reason.trim()} : {end_at: end, reason: reason.trim()};
        setBusy(true);
        setError('');
        try {
            await Client4.startFocus(request);
        } catch (e) {
            setBusy(false);
            setError((e as {message?: string}).message || formatMessage({id: 'fusion.focus.failed', defaultMessage: 'Focus mode couldn\'t be started.', description: 'Error in the focus mode dialog when the server refused to start it.'}));
            return;
        }
        toast(formatMessage({id: 'fusion.focus.started', defaultMessage: 'Focusing until {time}', description: 'Toast after focus mode starts; {time} is when it ends.'}, {time: endLabel}));
        onClose();
    };

    return (
        <Dialog
            label={title}
            onClose={onClose}
            onSubmit={submit}
            className={am('focus-dialog')}
        >
            <div className={am('pane')}>
                <h2>{title}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.focus.lead', defaultMessage: 'Do not disturb for a while: notifications are muted and your unread badges hide. When it ends, you see what you missed.', description: 'Focus mode dialog: what focus mode does.'})}</p>
                <div
                    className={am('focus-durations')}
                    role='radiogroup'
                    aria-label={formatMessage({id: 'fusion.focus.duration', defaultMessage: 'How long', description: 'Focus mode dialog: the label of the duration chips.'})}
                >
                    {FOCUS_DURATIONS.map((d) => (
                        <button
                            key={d.key}
                            type='button'
                            role='radio'
                            aria-checked={choice === d.key}
                            className={am('onboarding-option', {on: choice === d.key})}
                            onClick={() => setChoice(d.key)}
                        >
                            {formatMessage(d.label)}
                        </button>
                    ))}
                </div>
                {choice === 'custom' && (
                    <div className={am('focus-custom')}>
                        <input
                            type='datetime-local'
                            value={custom}
                            aria-label={formatMessage({id: 'fusion.focus.customLabel', defaultMessage: 'Focus until', description: 'Focus mode dialog: the label of the date and time field for a custom end.'})}
                            aria-invalid={!customValid}
                            onChange={(e) => setCustom(e.target.value)}
                        />
                    </div>
                )}
                <p
                    className={am('focus-end')}
                    aria-live='polite'
                >
                    {end ? formatMessage({id: 'fusion.focus.until', defaultMessage: 'Until {time}', description: 'Focus mode dialog: when the focus chosen would end.'}, {time: endLabel}) : formatMessage({id: 'fusion.focus.customInvalid', defaultMessage: 'Choose a time from a minute to a week from now.', description: 'Focus mode dialog: the custom end is in the past, too soon or too far.'})}
                </p>
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.focus.reason', defaultMessage: 'Reason (optional)', description: 'Focus mode dialog: the label of the reason field; the reason becomes your custom status.'})}</span>
                    <input
                        type='text'
                        value={reason}
                        maxLength={FOCUS_REASON_MAX}
                        placeholder={formatMessage({id: 'fusion.focus.reasonPlaceholder', defaultMessage: 'Working on the new release', description: 'Focus mode dialog: an example reason, in the empty reason field.'})}
                        onChange={(e) => setReason(e.target.value)}
                    />
                </label>
                {error && (
                    <p
                        className={am('field-error')}
                        role='alert'
                    >
                        {error}
                    </p>
                )}
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.focus.cancel', defaultMessage: 'Cancel', description: 'Focus mode dialog: closes it without starting focus mode.'})}
                    </button>
                    <button
                        type='submit'
                        className={am('btn', 'primary')}
                        disabled={busy || !end}
                    >
                        {formatMessage({id: 'fusion.focus.start', defaultMessage: 'Start', description: 'Focus mode dialog: starts focus mode.'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
