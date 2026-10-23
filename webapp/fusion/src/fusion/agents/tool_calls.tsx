// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {AIToolCallStatus, AIToolCallSummary, AIToolKind} from '@mattermost/types/ai_tools';
import {POST_PROP_AGENT_TOOL_CALLS} from '@mattermost/types/ai_tools';
import type {Post} from '@mattermost/types/posts';

import {Client4} from 'mattermost-redux/client';
import {getCurrentUserId, getUser} from 'mattermost-redux/selectors/entities/users';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// The tool calls an agent's reply waits on (Antimatter): what the agent wants to do, with the exact arguments the
// server recorded. Only the person the agent answered may confirm or decline, once; everyone else sees where it
// stands. The server checks all of it again: the buttons are only a way to answer.

const STATUSES: AIToolCallStatus[] = ['pending', 'done', 'failed', 'declined', 'expired'];

// toolCallsOf reads a post's tool calls, keeping only well-formed ones.
export function toolCallsOf(post: Post): AIToolCallSummary[] {
    const raw = post.props?.[POST_PROP_AGENT_TOOL_CALLS];
    if (!Array.isArray(raw)) {
        return [];
    }
    return raw.filter((c): c is AIToolCallSummary => Boolean(c) && typeof c.id === 'string' && typeof c.tool === 'string' && STATUSES.includes(c.status));
}

// prettyArguments shows a call's arguments indented when they're JSON, as given otherwise.
export function prettyArguments(args: string): string {
    try {
        const parsed = JSON.parse(args);
        if (parsed && typeof parsed === 'object' && Object.keys(parsed).length === 0) {
            return '';
        }
        return JSON.stringify(parsed, null, 2);
    } catch {
        return args;
    }
}

type Props = {
    post: Post;
};

export default function AgentToolCalls({post}: Props) {
    const calls = toolCallsOf(post);
    if (calls.length === 0) {
        return null;
    }
    return (
        <div className={am('agent-tool-calls')}>
            {calls.map((call) => (
                <ToolCall
                    key={call.id}
                    call={call}
                />
            ))}
        </div>
    );
}

function ToolCall({call}: {call: AIToolCallSummary}) {
    const {formatMessage} = useIntl();
    const me = useSelector(getCurrentUserId);
    const person = useSelector((state: GlobalState) => getUser(state, call.user_id));
    const [answered, setAnswered] = useState<AIToolCallStatus | null>(null);
    const [busy, setBusy] = useState(false);
    const [failed, setFailed] = useState(false);

    // What the server said last wins once the reply is updated; meanwhile, the answer just given.
    const status = call.status === 'pending' && answered ? answered : call.status;
    const mine = call.user_id === me;
    const args = prettyArguments(call.arguments);

    const answer = async (confirm: boolean) => {
        setBusy(true);
        setFailed(false);
        try {
            const summary = await Client4.answerAIToolCall(call.id, confirm);
            setAnswered(summary.status);
        } catch {
            setFailed(true);
        } finally {
            setBusy(false);
        }
    };

    const kindLabel: Record<AIToolKind, string> = {
        read: formatMessage({id: 'fusion.agentTools.kindRead', defaultMessage: 'Reads'}),
        write: formatMessage({id: 'fusion.agentTools.kindWrite', defaultMessage: 'Changes something here'}),
        external_write: formatMessage({id: 'fusion.agentTools.kindExternal', defaultMessage: 'Changes something in another service'}),
    };
    const statusLabel: Record<AIToolCallStatus, string> = {
        pending: mine ? formatMessage({id: 'fusion.agentTools.pendingYou', defaultMessage: 'Waiting for you'}) : formatMessage({id: 'fusion.agentTools.pendingOther', defaultMessage: 'Waiting for {name}'}, {name: person ? '@' + person.username : ''}),
        done: formatMessage({id: 'fusion.agentTools.done', defaultMessage: 'Done'}),
        failed: formatMessage({id: 'fusion.agentTools.failed', defaultMessage: 'Didn\'t work'}),
        declined: formatMessage({id: 'fusion.agentTools.declined', defaultMessage: 'Declined'}),
        expired: formatMessage({id: 'fusion.agentTools.expired', defaultMessage: 'Expired'}),
    };

    return (
        <div
            className={am('agent-tool-call', status)}
            data-testid='agent-tool-call'
        >
            <div className={am('atc-head')}>
                <span className={am('atc-ic')}>
                    <Icon
                        name='plug'
                        size='sm'
                    />
                </span>
                <span className={am('atc-title')}>
                    <b>{call.title || call.tool}</b>
                    <span className={am('atc-sub')}>
                        <span className={am('mono')}>{call.tool}</span>
                        {' · '}
                        {kindLabel[call.kind] ?? kindLabel.write}
                    </span>
                </span>
                {(status !== 'pending' || !mine) && <span className={am('atc-status', status)}>{statusLabel[status]}</span>}
            </div>
            {args && (
                <pre
                    className={am('atc-args')}
                    aria-label={formatMessage({id: 'fusion.agentTools.arguments', defaultMessage: 'What it will be called with'})}
                >
                    {args}
                </pre>
            )}
            {status === 'pending' && mine && (
                <div className={am('atc-actions')}>
                    {failed && <span className={am('atc-error')}>{formatMessage({id: 'fusion.agentTools.answerFailed', defaultMessage: 'Your answer couldn\'t be sent. Try again.'})}</span>}
                    <button
                        type='button'
                        className={am('btn', 'small')}
                        disabled={busy}
                        onClick={() => answer(false)}
                    >
                        {formatMessage({id: 'fusion.agentTools.decline', defaultMessage: 'Decline'})}
                    </button>
                    <button
                        type='button'
                        className={am('btn', 'primary', 'small')}
                        disabled={busy}
                        onClick={() => answer(true)}
                    >
                        {formatMessage({id: 'fusion.agentTools.confirm', defaultMessage: 'Confirm'})}
                    </button>
                </div>
            )}
        </div>
    );
}
