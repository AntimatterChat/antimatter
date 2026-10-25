// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {screen} from '@testing-library/react';
import React from 'react';

import type {Post} from '@mattermost/types/posts';

import {renderWithContext} from 'tests/react_testing_utils';

import Cards, {MessageAttachmentCard, sparkline} from './cards';

const post = (attachment: Record<string, unknown>) => ({
    id: 'p1',
    type: 'slack_attachment',
    message: '',
    create_at: 0,
    user_id: 'u1',
    props: {attachments: [{title: 'Pipeline #4821 failed on main', ...attachment}]},
}) as unknown as Post;

describe('Cards', () => {
    test('draws a pipeline\'s stages as pills, unknown states as pending', () => {
        renderWithContext(
            <Cards post={post({stages: [{name: 'build', status: 'passed'}, {name: 'test', status: 'failed'}, {name: 'flash', status: 'weird'}, {name: '', status: 'passed'}]})}/>,
        );
        const pills = screen.getByRole('list', {name: 'Stages'}).querySelectorAll('[role="listitem"]');
        expect(pills).toHaveLength(3);
        expect(pills[0]).toHaveClass('am-stage-pill', 'am-passed');
        expect(pills[0]).toHaveTextContent('build');
        expect(pills[1]).toHaveClass('am-failed');
        expect(pills[2]).toHaveClass('am-pending');
    });

    test('draws a chart with its threshold, and nothing for too few values', () => {
        const {rerender} = renderWithContext(
            <Cards post={post({color: '#F23F43', chart: {values: [2, 4, 13], threshold: 10, label: 'Vacuum pressure, last hour'}})}/>,
        );
        const chart = screen.getByRole('img', {name: 'Vacuum pressure, last hour'});
        expect(chart.querySelector('polyline')).toHaveAttribute('points');
        expect(chart.querySelector('line')).toBeInTheDocument();

        rerender(<Cards post={post({chart: {values: [2]}})}/>);
        expect(screen.queryByRole('img')).not.toBeInTheDocument();
    });

    test('shows a field with a title and an empty value, and drops an empty field', () => {
        renderWithContext(
            <Cards post={post({fields: [{title: 'Owner', value: ''}, {title: '', value: ''}, {title: 'Build', value: 4821}]})}/>,
        );
        const terms = screen.getAllByRole('term');
        expect(terms.map((t) => t.textContent)).toEqual(['Owner', 'Build']);
        expect(screen.getAllByRole('definition')).toHaveLength(1);
        expect(screen.getByRole('definition')).toHaveTextContent('4821');
    });

    test('scales a chart to include its threshold', () => {
        const line = sparkline([1, 2, 3], 10, 100, 50);
        const ys = line.points.split(' ').map((p) => Number(p.split(',')[1]));

        // The threshold is above every value: higher on screen means a smaller y.
        expect(line.threshold).toBeLessThan(Math.min(...ys));
        expect(line.last.x).toBeCloseTo(92);
    });
});

// MessageAttachmentCard is published on window.WebappUtils.cards for apps that aren't rendering a post
// (ASSISTANT_DISPLAY.md #2): an assistant result, with a channel but no post of its own.
describe('MessageAttachmentCard', () => {
    test('draws a chart and fields with no post, and drops actions', () => {
        renderWithContext(
            <MessageAttachmentCard
                attachments={[{
                    title: 'CPU on www-1',
                    color: '#3E8EDE',
                    fields: [{title: 'Now', value: '4.1%', short: true}, {title: 'Peak', value: '5.8%', short: true}],
                    chart: {values: [3.1, 3.4, 4.0, 4.1], threshold: 80, label: 'CPU %'},
                    actions: [{id: 'ack', name: 'Acknowledge', type: 'button'}],
                }]}
                summary='CPU on www-1: 4.1% now'
            />,
        );

        expect(screen.getByRole('group', {name: 'CPU on www-1: 4.1% now'})).toBeInTheDocument();
        expect(screen.getByText('CPU on www-1')).toBeInTheDocument();
        const chart = screen.getByRole('img', {name: 'CPU %'});
        expect(chart.querySelector('polyline')).toHaveAttribute('points');
        const terms = screen.getAllByRole('term');
        expect(terms.map((t) => t.textContent)).toEqual(['Now', 'Peak']);
        expect(screen.queryByRole('button', {name: 'Acknowledge'})).not.toBeInTheDocument();
    });

    test('falls back to an empty group for a non-array attachments value', () => {
        renderWithContext(
            <MessageAttachmentCard
                attachments={'not-an-array' as unknown as Parameters<typeof MessageAttachmentCard>[0]['attachments']}
                summary='Nothing to show'
            />,
        );
        expect(screen.getByRole('group', {name: 'Nothing to show'})).toBeEmptyDOMElement();
    });
});
