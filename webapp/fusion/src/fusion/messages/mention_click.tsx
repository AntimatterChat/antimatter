// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useState} from 'react';
import {useStore} from 'react-redux';

import {getUsersByUsername} from 'mattermost-redux/selectors/entities/users';

import UserPopover from 'fusion/popovers/user_popover';
import {getMentionDetails} from 'utils/post_utils';

import type {GlobalState} from 'types/store';

// useMentionClick makes the @mentions of a message body open the Fusion profile card instead of the classic one:
// put the handler on the body as onClickCapture, and render the card it returns.
export function useMentionClick(): [React.ReactNode, (e: React.MouseEvent) => void] {
    const store = useStore<GlobalState>();
    const [target, setTarget] = useState<{userId: string; anchor: HTMLElement} | null>(null);

    const onClickCapture = useCallback((e: React.MouseEvent) => {
        const mention = (e.target as Element).closest?.('[data-mention]');
        const trigger = mention?.querySelector<HTMLElement>('.mention-link')?.closest<HTMLElement>('button') || null;
        if (!mention || !trigger || !trigger.contains(e.target as Node)) {
            return;
        }
        const user = getMentionDetails(getUsersByUsername(store.getState()), mention.getAttribute('data-mention') || '');
        if (!user) {
            return;
        }

        // The classic mention's own popover never sees the click.
        e.preventDefault();
        e.stopPropagation();
        setTarget({userId: user.id, anchor: trigger});
    }, [store]);

    const node = target ? (
        <UserPopover
            userId={target.userId}
            anchor={target.anchor}
            onClose={() => setTarget(null)}
        />
    ) : null;
    return [node, onClickCapture];
}
