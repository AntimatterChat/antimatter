// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {topicPath} from './paths';

describe('fusion/utils/paths', () => {
    test('topicPath points at the topic under the channel (Antimatter forums)', () => {
        expect(topicPath('lumen-labs', {name: 'beam-questions'}, 'r1')).toBe('/lumen-labs/channels/beam-questions/topics/r1');
    });
});
