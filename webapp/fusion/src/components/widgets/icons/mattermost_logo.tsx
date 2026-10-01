// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React from 'react';
import {useIntl} from 'react-intl';

// Antimatter mark: a particle and its antiparticle on mirrored orbits.
export default function MattermostLogo(props: React.HTMLAttributes<HTMLSpanElement>) {
    const {formatMessage} = useIntl();
    return (
        <span {...props}>
            <svg
                version='1.1'
                viewBox='0 0 64 64'
                role='img'
                aria-label={formatMessage({id: 'generic_icons.mattermost', defaultMessage: 'Antimatter Logo'})}
            >
                <ellipse
                    cx='32'
                    cy='32'
                    rx='25'
                    ry='9.5'
                    fill='none'
                    stroke='#6D28D9'
                    strokeWidth='4.5'
                    transform='rotate(45 32 32)'
                />
                <ellipse
                    cx='32'
                    cy='32'
                    rx='25'
                    ry='9.5'
                    fill='none'
                    stroke='#0891B2'
                    strokeWidth='4.5'
                    transform='rotate(-45 32 32)'
                />
                <path
                    d='M32 20.5 Q33.9 30.1 43.5 32 Q33.9 33.9 32 43.5 Q30.1 33.9 20.5 32 Q30.1 30.1 32 20.5 Z'
                    fill='#F59E0B'
                />
                <circle
                    cx='14.32'
                    cy='14.32'
                    r='5.5'
                    fill='#6D28D9'
                />
                <circle
                    cx='49.68'
                    cy='14.32'
                    r='4.25'
                    fill='#FFFFFF'
                    stroke='#0891B2'
                    strokeWidth='2.5'
                />
            </svg>
        </span>
    );
}
