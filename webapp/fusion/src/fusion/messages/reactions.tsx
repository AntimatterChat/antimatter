// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Reaction} from '@mattermost/types/reactions';

import {getMissingProfilesByIds} from 'mattermost-redux/actions/users';
import {makeGetReactionsForPost} from 'mattermost-redux/selectors/entities/posts';
import {getTeammateNameDisplaySetting} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUserId, getUsers} from 'mattermost-redux/selectors/entities/users';
import {displayUsername} from 'mattermost-redux/utils/user_utils';

import {toggleReaction} from 'actions/post_actions';

import RenderEmoji from 'components/emoji/render_emoji';

import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

type Group = {name: string; userIds: string[]; mine: boolean; first: number};

// How many people a reaction's tooltip names before saying how many others reacted.
const NAMED = 10;

// Reactions are the emoji pills under a message; clicking one adds or removes your reaction, and hovering it tells who
// reacted, as in Discord.
export default function Reactions({postId}: {postId: string}) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const dispatch = useDispatch();
    const getReactions = useMemo(() => makeGetReactionsForPost(), []);
    const reactions = useSelector((state: GlobalState) => getReactions(state, postId));
    const me = useSelector(getCurrentUserId);
    const users = useSelector(getUsers);
    const nameDisplay = useSelector(getTeammateNameDisplaySetting);

    const groups = useMemo(() => {
        const byName = new Map<string, Group>();
        Object.values(reactions || {}).forEach((r: Reaction) => {
            const g = byName.get(r.emoji_name) || {name: r.emoji_name, userIds: [], mine: false, first: r.create_at};
            g.userIds.push(r.user_id);
            g.mine = g.mine || r.user_id === me;
            g.first = Math.min(g.first, r.create_at);
            byName.set(r.emoji_name, g);
        });
        return [...byName.values()].sort((a, b) => a.first - b.first);
    }, [reactions, me]);

    if (!groups.length) {
        return null;
    }

    // The tooltip names you first, then the others in the order they reacted.
    const reactedBy = (g: Group) => {
        const others = g.userIds.filter((id) => id !== me).map((id) => (users[id] ? displayUsername(users[id], nameDisplay) : '')).filter(Boolean);
        const names = g.mine ? [formatMessage({id: 'fusion.reactions.you', defaultMessage: 'You'}), ...others] : others;
        const unnamed = (g.userIds.length - names.length) + Math.max(names.length - NAMED, 0);
        const listed = names.slice(0, NAMED);
        if (unnamed > 0) {
            listed.push(formatMessage({id: 'fusion.reactions.others', defaultMessage: '{count, plural, one {# other} other {# others}}'}, {count: unnamed}));
        }
        return formatMessage({id: 'fusion.reactions.reactedBy', defaultMessage: '{names} reacted with :{name}:'}, {names: intl.formatList(listed, {type: 'conjunction'}), name: g.name});
    };

    // Who reacted is only needed on hover: load the people who aren't then.
    const loadReactors = () => {
        const missing = groups.flatMap((g) => g.userIds).filter((id) => !users[id]);
        if (missing.length) {
            dispatch(getMissingProfilesByIds(missing));
        }
    };

    return (
        <div
            className={am('reactions')}
            onMouseEnter={loadReactors}
        >
            {groups.map((g) => (
                <button
                    key={g.name}
                    className={am('react', {mine: g.mine})}
                    aria-pressed={g.mine}
                    data-am-tip={reactedBy(g)}
                    aria-label={formatMessage({id: 'fusion.reactions.label', defaultMessage: ':{name}: {count}, {mine, select, true {remove your reaction} other {react}}'}, {name: g.name, count: g.userIds.length, mine: String(g.mine)})}
                    onClick={() => dispatch(toggleReaction(postId, g.name))}
                >
                    <RenderEmoji
                        emojiName={g.name}
                        size={16}
                    />
                    <span>{g.userIds.length}</span>
                </button>
            ))}
        </div>
    );
}
