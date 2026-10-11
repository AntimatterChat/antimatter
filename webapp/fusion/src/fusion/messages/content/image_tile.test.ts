// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {imageTileStyle} from './image_tile';

describe('imageTileStyle', () => {
    test('fits a tile to the image, so that nothing is cropped', () => {
        expect(imageTileStyle(960, 540)).toEqual({aspectRatio: '960 / 540', width: 'min(340px, 100%, 569px)'});
        expect(imageTileStyle(768, 768)).toEqual({aspectRatio: '768 / 768', width: 'min(340px, 100%, 320px)'});
        expect(imageTileStyle(600, 1200)).toEqual({aspectRatio: '600 / 1200', width: 'min(340px, 100%, 160px)'});
    });

    test('keeps the default tile for unknown sizes', () => {
        expect(imageTileStyle()).toBeUndefined();
        expect(imageTileStyle(0, 100)).toBeUndefined();
    });
});
