// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {GlobalState} from '@mattermost/types/store';

import {getConfig} from 'mattermost-redux/selectors/entities/general';

export function isAnonymousURLEnabled(state: GlobalState): boolean {
    return getConfig(state).UseAnonymousURLs === 'true';
}
