// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {A11yAttributeNames, A11yClassNames} from 'utils/constants';

// The panes F6 (Ctrl/Cmd+F6 in a browser) and Ctrl+` cycle through, in this order, with Mattermost's accessibility
// controller: each pane carries its region class and place.
const ORDER = {
    rail: 1,
    sidebar: 2,
    header: 3,
    messages: 4,
    composer: 5,
    rhs: 6,
};

export type Region = keyof typeof ORDER;

// a11yRegion gives a pane's element its region class and place, to spread on it after its own class.
export function a11yRegion(className: string, region: Region) {
    return {
        className: `${className} ${A11yClassNames.REGION}`,
        [A11yAttributeNames.SORT_ORDER]: ORDER[region],
    };
}
