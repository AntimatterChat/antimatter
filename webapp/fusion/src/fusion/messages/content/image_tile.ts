// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type React from 'react';

// The largest an image tile of a message gets.
export const TILE_MAX_WIDTH = 340;
export const TILE_MAX_HEIGHT = 320;

// imageTileStyle sizes an image's tile to the image's shape, within TILE_MAX_WIDTH × TILE_MAX_HEIGHT and the message's
// width, so that the picture shows whole (a meme's captions, a portrait photo); unknown sizes keep the tile's 4:3.
export function imageTileStyle(width?: number, height?: number): React.CSSProperties | undefined {
    if (!width || !height || width <= 0 || height <= 0) {
        return undefined;
    }
    const fit = Math.round((TILE_MAX_HEIGHT * width) / height);
    return {aspectRatio: `${width} / ${height}`, width: `min(${TILE_MAX_WIDTH}px, 100%, ${fit}px)`};
}
