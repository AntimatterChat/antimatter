// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo} from 'react';
import {useSelector} from 'react-redux';

import {isCustomStatusEnabled, isCustomStatusExpired, makeGetCustomStatus} from 'selectors/views/custom_status';

import RenderEmoji from 'components/emoji/render_emoji';

import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// StatusEmoji is the emoji of someone's custom status, after their name, with its text as tooltip; nothing when they
// have none, it expired, or custom statuses are off.
export default function StatusEmoji({userId, size = 14}: {userId?: string; size?: number}) {
    const getCustomStatus = useMemo(() => makeGetCustomStatus(), []);
    const status = useSelector((state: GlobalState) => {
        if (!userId || !isCustomStatusEnabled(state)) {
            return undefined;
        }
        const custom = getCustomStatus(state, userId);
        return custom && !isCustomStatusExpired(state, custom) ? custom : undefined;
    });

    if (!status?.emoji) {
        return null;
    }
    return (
        <span
            className={am('status-emoji')}
            data-am-tip={status.text || undefined}
            aria-label={status.text || status.emoji}
        >
            <RenderEmoji
                emojiName={status.emoji}
                size={size}
            />
        </span>
    );
}
