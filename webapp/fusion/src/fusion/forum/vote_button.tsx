// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {Post} from '@mattermost/types/posts';

import {makeGetReactionsForPost} from 'mattermost-redux/selectors/entities/posts';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {toggleReaction} from 'actions/post_actions';

import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {countVotes, hasVoted} from './votes';

// VoteButton is a forum topic's or reply's 👍 vote (design §3.3): the question, an idea, or a Q&A answer. Hidden on
// your own post (the owner's decision: your own vote doesn't count, so there is nothing to show or toggle).
export default function VoteButton({post, className}: {post: Post; className?: string}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const me = useSelector(getCurrentUserId);
    const getReactionsForPost = useMemo(() => makeGetReactionsForPost(), []);
    const reactions = useSelector((state: GlobalState) => getReactionsForPost(state, post.id));
    if (post.user_id === me) {
        return null;
    }
    const votes = countVotes(reactions, post.user_id);
    const mine = hasVoted(reactions, me, post.user_id);
    const label = mine ? formatMessage({id: 'fusion.forum.voteRemove', defaultMessage: 'Remove your vote', description: 'Accessible label and tooltip of a forum topic\'s or reply\'s 👍 vote button, once you\'ve voted.'}) : formatMessage({id: 'fusion.forum.vote', defaultMessage: 'Vote for this', description: 'Accessible label and tooltip of a forum topic\'s or reply\'s 👍 vote button, before you\'ve voted.'});
    return (
        <button
            type='button'
            className={am('vote-btn', {on: mine}, className)}
            aria-pressed={mine}
            aria-label={label}
            title={label}
            onClick={(e) => {
                e.stopPropagation();
                dispatch(toggleReaction(post.id, '+1'));
            }}
        >
            <span aria-hidden='true'>{'👍'}</span>
            {votes}
        </button>
    );
}
