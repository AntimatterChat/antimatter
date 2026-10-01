// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';

import {am} from 'fusion/utils/class_names';

import type {IconName} from './icon_sprite';

type Props = {
    name: IconName;
    size?: 'sm' | 'xs';
    className?: string;
};

// Icon draws one of the Fusion UI's icons (see IconSprite).
export default function Icon({name, size, className}: Props) {
    return (
        <svg
            className={am('ic', size) + (className ? ' ' + className : '')}
            aria-hidden='true'
        >
            <use href={`#am-i-${name}`}/>
        </svg>
    );
}

export type {IconName};
