// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {defineMessage, useIntl} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';

import type {Channel} from '@mattermost/types/channels';

import {Client4} from 'mattermost-redux/client';

import {Dialog} from 'fusion/components/layer';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import Constants from 'utils/constants';

type Range = 'all' | 'month' | 'custom';

const RANGES: Array<{range: Range; label: MessageDescriptor}> = [
    {range: 'all', label: defineMessage({id: 'fusion.channelExport.allTime', defaultMessage: 'All time'})},
    {range: 'month', label: defineMessage({id: 'fusion.channelExport.lastMonth', defaultMessage: 'Last 30 days'})},
    {range: 'custom', label: defineMessage({id: 'fusion.channelExport.custom', defaultMessage: 'Custom'})},
];

const DAY = 24 * 60 * 60 * 1000;

// The value of a date input: the local day, as YYYY-MM-DD.
export function dayValue(time: number): string {
    const d = new Date(time);
    return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`;
}

// The start of a local day given as YYYY-MM-DD, in milliseconds, or 0 when there's no day; with end, the last
// millisecond of that day.
export function dayTime(value: string, end = false): number {
    const m = (/^(\d{4})-(\d{2})-(\d{2})$/).exec(value);
    if (!m) {
        return 0;
    }
    return new Date(Number(m[1]), Number(m[2]) - 1, Number(m[3]) + (end ? 1 : 0)).getTime() - (end ? 1 : 0);
}

// exportRange is the creation time range of the messages to export, in milliseconds; 0 leaves a bound open.
export function exportRange(range: Range, from: string, to: string, now = Date.now()): {since: number; until: number} {
    switch (range) {
    case 'month':
        return {since: now - (30 * DAY), until: 0};
    case 'custom':
        return {since: dayTime(from), until: dayTime(to, true)};
    default:
        return {since: 0, until: 0};
    }
}

function download(blob: Blob, filename: string) {
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    link.click();
    URL.revokeObjectURL(url);
}

// ChannelExportDialog downloads a channel's messages as a CSV file: all of them, the last 30 days' or those of a
// range of days.
export default function ChannelExportDialog({channel, onClose}: {channel: Channel; onClose: () => void}) {
    const {formatMessage} = useIntl();
    const toast = useToast();
    const [range, setRange] = useState<Range>('all');
    const [from, setFrom] = useState(() => dayValue(Date.now() - (30 * DAY)));
    const [to, setTo] = useState(() => dayValue(Date.now()));
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');

    const direct = channel.type === Constants.DM_CHANNEL || channel.type === Constants.GM_CHANNEL;
    const name = direct ? channel.display_name : '#' + channel.display_name;
    const {since, until} = exportRange(range, from, to);
    const backwards = range === 'custom' && since > 0 && until > 0 && since > until;

    const submit = async () => {
        if (busy || backwards) {
            return;
        }
        setBusy(true);
        setError('');
        try {
            const {blob, filename} = await Client4.exportChannel(channel.id, {since: since || undefined, until: until || undefined});
            download(blob, filename);
        } catch (err) {
            setBusy(false);
            setError((err as {message?: string}).message || formatMessage({id: 'fusion.channelExport.failed', defaultMessage: 'The channel couldn\'t be exported.'}));
            return;
        }
        toast(formatMessage({id: 'fusion.channelExport.done', defaultMessage: '{name} exported'}, {name}));
        onClose();
    };

    const title = formatMessage({id: 'fusion.channelExport.title', defaultMessage: 'Export {name}'}, {name});
    return (
        <Dialog
            label={title}
            onClose={onClose}
            onSubmit={submit}
        >
            <div className={am('pane')}>
                <h2>{title}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.channelExport.lead', defaultMessage: 'Download the messages as a file you can open in a spreadsheet: who wrote them, when, and their replies. Attachments are listed by name. Deleted messages aren\'t included.'})}</p>
                <div className={am('field')}>
                    <span>{formatMessage({id: 'fusion.channelExport.range', defaultMessage: 'Messages'})}</span>
                    <div
                        className={am('onboarding-options')}
                        role='radiogroup'
                        aria-label={formatMessage({id: 'fusion.channelExport.range', defaultMessage: 'Messages'})}
                    >
                        {RANGES.map((r) => (
                            <button
                                key={r.range}
                                type='button'
                                role='radio'
                                aria-checked={range === r.range}
                                className={am('onboarding-option', {on: range === r.range})}
                                onClick={() => setRange(r.range)}
                            >
                                {formatMessage(r.label)}
                            </button>
                        ))}
                    </div>
                </div>
                {range === 'custom' && (
                    <div className={am('export-range')}>
                        <label className={am('field')}>
                            <span>{formatMessage({id: 'fusion.channelExport.from', defaultMessage: 'From'})}</span>
                            <input
                                type='date'
                                value={from}
                                max={to || undefined}
                                onChange={(e) => setFrom(e.target.value)}
                            />
                        </label>
                        <label className={am('field')}>
                            <span>{formatMessage({id: 'fusion.channelExport.to', defaultMessage: 'To'})}</span>
                            <input
                                type='date'
                                value={to}
                                min={from || undefined}
                                onChange={(e) => setTo(e.target.value)}
                            />
                        </label>
                    </div>
                )}
                {backwards && <p className={am('field-error')}>{formatMessage({id: 'fusion.channelExport.backwards', defaultMessage: 'The first day is after the last one.'})}</p>}
                <div className={am('field')}>
                    <span>{formatMessage({id: 'fusion.channelExport.format', defaultMessage: 'Format'})}</span>
                    <div
                        className={am('onboarding-options')}
                        role='radiogroup'
                        aria-label={formatMessage({id: 'fusion.channelExport.format', defaultMessage: 'Format'})}
                    >
                        <button
                            type='button'
                            role='radio'
                            aria-checked={true}
                            className={am('onboarding-option', 'on')}
                        >
                            {formatMessage({id: 'fusion.channelExport.csv', defaultMessage: 'CSV'})}
                        </button>
                    </div>
                </div>
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
                        {formatMessage({id: 'fusion.channelExport.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button
                        type='submit'
                        className={am('btn', 'primary')}
                        disabled={busy || backwards}
                    >
                        {busy ? formatMessage({id: 'fusion.channelExport.exporting', defaultMessage: 'Exporting…'}) : formatMessage({id: 'fusion.channelExport.submit', defaultMessage: 'Export'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
