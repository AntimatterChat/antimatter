// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {screen} from '@testing-library/react';
import React from 'react';

import type {Post} from '@mattermost/types/posts';

import {renderWithContext} from 'tests/react_testing_utils';

import Cards, {sparkline} from './cards';

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

    test('scales a chart to include its threshold', () => {
        const line = sparkline([1, 2, 3], 10, 100, 50);
        const ys = line.points.split(' ').map((p) => Number(p.split(',')[1]));

        // The threshold is above every value: higher on screen means a smaller y.
        expect(line.threshold).toBeLessThan(Math.min(...ys));
        expect(line.last.x).toBeCloseTo(92);
    });
});
