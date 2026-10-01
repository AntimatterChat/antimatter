// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Reaction} from '@mattermost/types/reactions';

import {makeGetReactionsForPost} from 'mattermost-redux/selectors/entities/posts';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {toggleReaction} from 'actions/post_actions';

import RenderEmoji from 'components/emoji/render_emoji';

import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

type Group = {name: string; count: number; mine: boolean; first: number};

// Reactions are the emoji pills under a message; clicking one adds or removes your reaction.
export default function Reactions({postId}: {postId: string}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const getReactions = useMemo(() => makeGetReactionsForPost(), []);
    const reactions = useSelector((state: GlobalState) => getReactions(state, postId));
    const me = useSelector(getCurrentUserId);

    const groups = useMemo(() => {
        const byName = new Map<string, Group>();
        Object.values(reactions || {}).forEach((r: Reaction) => {
            const g = byName.get(r.emoji_name) || {name: r.emoji_name, count: 0, mine: false, first: r.create_at};
            g.count += 1;
            g.mine = g.mine || r.user_id === me;
            g.first = Math.min(g.first, r.create_at);
            byName.set(r.emoji_name, g);
        });
        return [...byName.values()].sort((a, b) => a.first - b.first);
    }, [reactions, me]);

    if (!groups.length) {
        return null;
    }
    return (
        <div className={am('reactions')}>
            {groups.map((g) => (
                <button
                    key={g.name}
                    className={am('react', {mine: g.mine})}
                    aria-pressed={g.mine}
                    title={`:${g.name}:`}
                    aria-label={formatMessage({id: 'fusion.reactions.label', defaultMessage: ':{name}: {count}, {mine, select, true {remove your reaction} other {react}}'}, {name: g.name, count: g.count, mine: String(g.mine)})}
                    onClick={() => dispatch(toggleReaction(postId, g.name))}
                >
                    <RenderEmoji
                        emojiName={g.name}
                        size={16}
                    />
                    <span>{g.count}</span>
                </button>
            ))}
        </div>
    );
}
