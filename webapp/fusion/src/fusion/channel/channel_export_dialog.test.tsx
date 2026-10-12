// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';

import type {Channel} from '@mattermost/types/channels';

import {Client4} from 'mattermost-redux/client';

import {LAYER_ID} from 'fusion/components/layer';
import Toasts from 'fusion/components/toast';
import {renderWithContext, screen, userEvent, waitFor} from 'tests/react_testing_utils';

import ChannelExportDialog, {REVOKE_DELAY, dayTime, dayValue, download, exportRange} from './channel_export_dialog';

const channel = {id: 'lab', name: 'lab', display_name: 'Lab', team_id: 'team', type: 'O', delete_at: 0} as Channel;

describe('export ranges', () => {
    const now = new Date(2026, 9, 12, 15, 30).getTime();

    test('all time has no bounds', () => {
        expect(exportRange('all', '', '', now)).toEqual({since: 0, until: 0});
    });

    test('the last 30 days', () => {
        expect(exportRange('month', '', '', now)).toEqual({since: now - (30 * 24 * 60 * 60 * 1000), until: 0});
    });

    test('a custom range covers whole local days, and an empty day leaves its bound open', () => {
        expect(exportRange('custom', '2026-10-01', '2026-10-12', now)).toEqual({
            since: new Date(2026, 9, 1).getTime(),
            until: new Date(2026, 9, 13).getTime() - 1,
        });
        expect(exportRange('custom', '', '2026-10-12', now).since).toBe(0);
        expect(dayTime('not a day')).toBe(0);
    });

    test('days are written as date inputs want them', () => {
        expect(dayValue(new Date(2026, 0, 5, 23, 59).getTime())).toBe('2026-01-05');
    });
});

describe('download', () => {
    afterEach(() => {
        jest.useRealTimers();
        jest.restoreAllMocks();
    });

    test('clicks a link to the file in the document, and revokes its URL later', () => {
        jest.useFakeTimers();
        window.URL.createObjectURL = jest.fn().mockReturnValue('blob:export');
        window.URL.revokeObjectURL = jest.fn();
        let inDocument = false;
        const click = jest.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(function(this: HTMLAnchorElement) {
            inDocument = document.body.contains(this);
        });

        download(new Blob(['Post Id\n']), 'lab.csv');

        expect(click).toHaveBeenCalledTimes(1);
        const link = click.mock.instances[0] as unknown as HTMLAnchorElement;
        expect(link.download).toBe('lab.csv');
        expect(link.href).toBe('blob:export');
        expect(inDocument).toBe(true);
        expect(document.body.contains(link)).toBe(false);

        expect(window.URL.revokeObjectURL).not.toHaveBeenCalled();
        jest.advanceTimersByTime(REVOKE_DELAY);
        expect(window.URL.revokeObjectURL).toHaveBeenCalledWith('blob:export');
    });
});

describe('ChannelExportDialog', () => {
    let exportChannel: jest.SpyInstance;
    let click: jest.SpyInstance;

    beforeEach(() => {
        const layer = document.createElement('div');
        layer.id = LAYER_ID;
        document.body.appendChild(layer);
        exportChannel = jest.spyOn(Client4, 'exportChannel').mockResolvedValue({blob: new Blob(['Post Id\n']), filename: 'lab-2026-10-12.csv'});
        window.URL.createObjectURL = jest.fn().mockReturnValue('blob:export');
        window.URL.revokeObjectURL = jest.fn();
        click = jest.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => undefined);
    });

    afterEach(() => {
        document.getElementById(LAYER_ID)?.remove();
        jest.restoreAllMocks();
    });

    const render = () => {
        const onClose = jest.fn();
        renderWithContext(
            <>
                <ChannelExportDialog
                    channel={channel}
                    onClose={onClose}
                />
                <Toasts/>
            </>,
        );
        return onClose;
    };

    test('exports all the messages as a CSV download', async () => {
        const onClose = render();
        expect(screen.getByRole('dialog', {name: 'Export #Lab'})).toBeInTheDocument();
        expect(screen.getByRole('radio', {name: 'All time'})).toHaveAttribute('aria-checked', 'true');
        expect(screen.getByRole('radio', {name: 'CSV'})).toHaveAttribute('aria-checked', 'true');

        await userEvent.click(screen.getByRole('button', {name: 'Export'}));

        await waitFor(() => expect(onClose).toHaveBeenCalled());
        expect(exportChannel).toHaveBeenCalledWith('lab', {since: undefined, until: undefined, signal: expect.any(AbortSignal)});
        expect(click).toHaveBeenCalled();
        expect((click.mock.instances[0] as unknown as HTMLAnchorElement).download).toBe('lab-2026-10-12.csv');
        expect(screen.getByRole('status')).toHaveTextContent('#Lab exported');
    });

    test('the last 30 days', async () => {
        render();
        await userEvent.click(screen.getByRole('radio', {name: 'Last 30 days'}));
        await userEvent.click(screen.getByRole('button', {name: 'Export'}));

        await waitFor(() => expect(exportChannel).toHaveBeenCalled());
        const {since, until} = exportChannel.mock.calls[0][1];
        expect(Date.now() - since).toBeGreaterThanOrEqual(30 * 24 * 60 * 60 * 1000);
        expect(Date.now() - since).toBeLessThan((30 * 24 * 60 * 60 * 1000) + 60000);
        expect(until).toBeUndefined();
    });

    test('a custom range of days, which can\'t run backwards', async () => {
        render();
        await userEvent.click(screen.getByRole('radio', {name: 'Custom'}));
        const from = screen.getByLabelText('From');
        const to = screen.getByLabelText('To');

        await userEvent.clear(from);
        await userEvent.type(from, '2026-10-05');
        await userEvent.clear(to);
        await userEvent.type(to, '2026-10-01');
        expect(screen.getByText('The first day is after the last one.')).toBeInTheDocument();
        expect(screen.getByRole('button', {name: 'Export'})).toBeDisabled();

        await userEvent.clear(to);
        await userEvent.type(to, '2026-10-07');
        await userEvent.click(screen.getByRole('button', {name: 'Export'}));

        await waitFor(() => expect(exportChannel).toHaveBeenCalledWith('lab', {
            since: new Date(2026, 9, 5).getTime(),
            until: new Date(2026, 9, 8).getTime() - 1,
            signal: expect.any(AbortSignal),
        }));
    });

    test('the server\'s refusal is shown, and the dialog stays open', async () => {
        exportChannel.mockRejectedValue(new Error('There are more than 50000 messages to export. Choose a shorter date range.'));
        const onClose = render();
        await userEvent.click(screen.getByRole('button', {name: 'Export'}));

        expect(await screen.findByRole('alert')).toHaveTextContent('There are more than 50000 messages to export.');
        expect(onClose).not.toHaveBeenCalled();
        expect(click).not.toHaveBeenCalled();
        expect(screen.getByRole('button', {name: 'Export'})).toBeEnabled();
    });

    test('closing the dialog cancels the export: no download, no toast', async () => {
        let finish: (file: {blob: Blob; filename: string}) => void = () => {};
        exportChannel.mockImplementation(() => new Promise((resolve) => {
            finish = resolve;
        }));

        function Harness() {
            const [open, setOpen] = useState(true);
            return open ? (
                <ChannelExportDialog
                    channel={channel}
                    onClose={() => setOpen(false)}
                />
            ) : null;
        }
        renderWithContext(
            <>
                <Harness/>
                <Toasts/>
            </>,
        );

        await userEvent.click(screen.getByRole('button', {name: 'Export'}));
        await waitFor(() => expect(exportChannel).toHaveBeenCalled());
        const {signal} = exportChannel.mock.calls[0][1];
        expect(signal.aborted).toBe(false);

        await userEvent.click(screen.getByRole('button', {name: 'Cancel'}));
        expect(screen.queryByRole('dialog')).not.toBeInTheDocument();
        expect(signal.aborted).toBe(true);

        // Even if the answer comes anyway, it's ignored.
        finish({blob: new Blob(['Post Id\n']), filename: 'lab-2026-10-12.csv'});
        await new Promise((resolve) => setTimeout(resolve, 0));
        expect(click).not.toHaveBeenCalled();
        expect(screen.queryByRole('status')).not.toBeInTheDocument();
    });
});
