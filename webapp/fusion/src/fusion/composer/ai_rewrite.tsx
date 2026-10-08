// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {getAgents as fetchAgents} from 'mattermost-redux/actions/agents';
import {Client4} from 'mattermost-redux/client';
import {getAgents} from 'mattermost-redux/selectors/entities/agents';

import {RewriteAction} from 'components/advanced_text_editor/rewrite_action';
import {useSelectedAgent} from 'components/common/agents';

import {Popover} from 'fusion/components/layer';
import {MenuHeading, MenuItem} from 'fusion/components/menu';
import {am} from 'fusion/utils/class_names';

const ACTIONS: Array<[RewriteAction, {id: string; defaultMessage: string}]> = [
    [RewriteAction.IMPROVE_WRITING, {id: 'fusion.ai.improve', defaultMessage: 'Improve the writing'}],
    [RewriteAction.FIX_SPELLING, {id: 'fusion.ai.spelling', defaultMessage: 'Fix spelling and grammar'}],
    [RewriteAction.SHORTEN, {id: 'fusion.ai.shorten', defaultMessage: 'Make it shorter'}],
    [RewriteAction.ELABORATE, {id: 'fusion.ai.elaborate', defaultMessage: 'Make it longer'}],
    [RewriteAction.SIMPLIFY, {id: 'fusion.ai.simplify', defaultMessage: 'Simplify'}],
    [RewriteAction.SUMMARIZE, {id: 'fusion.ai.summarize', defaultMessage: 'Summarize'}],
];

// useAgentsAvailable says whether an AI agent can rewrite messages (the Agents plugin, with an agent).
export function useAgentsAvailable(): boolean {
    const dispatch = useDispatch();
    const agents = useSelector(getAgents);
    useEffect(() => {
        dispatch(fetchAgents());
    }, [dispatch]);
    return Boolean(agents?.length);
}

type Props = {
    anchor: HTMLElement | null;
    message: string;
    rootId: string;
    onRewritten: (message: string, original: string) => void;
    onError: (message: string) => void;
    onBusy: (busy: boolean) => void;
    onClose: () => void;
};

// AiRewriteMenu has an AI agent rewrite the message being written: the classic editor's AI actions, or what to do
// in your own words (which also writes a message from nothing).
export default function AiRewriteMenu({anchor, message, rootId, onRewritten, onError, onBusy, onClose}: Props) {
    const {formatMessage} = useIntl();
    const agents = useSelector(getAgents);
    const [agentId] = useSelectedAgent(agents);
    const [prompt, setPrompt] = useState('');

    const rewrite = async (action: RewriteAction, custom?: string) => {
        onClose();
        onBusy(true);
        try {
            const result = await Client4.getAIRewrittenMessage(agentId, message, action, custom, rootId);
            onRewritten(result, message);
        } catch (e) {
            onError((e as {message?: string})?.message || formatMessage({id: 'fusion.ai.failed', defaultMessage: 'The AI couldn\'t rewrite the message'}));
        } finally {
            onBusy(false);
        }
    };

    return (
        <Popover
            anchor={anchor}
            placement='above'
            className='plus-pop ai-pop'
            role='menu'
            label={formatMessage({id: 'fusion.ai.label', defaultMessage: 'Rewrite with AI'})}
            onClose={onClose}
        >
            <MenuHeading>{formatMessage({id: 'fusion.ai.label', defaultMessage: 'Rewrite with AI'})}</MenuHeading>
            <form
                className={am('ai-prompt')}
                onSubmit={(e) => {
                    e.preventDefault();
                    if (prompt.trim()) {
                        rewrite(RewriteAction.CUSTOM, prompt.trim());
                    }
                }}
            >
                <input
                    autoFocus={true}
                    value={prompt}
                    placeholder={message.trim() ? formatMessage({id: 'fusion.ai.promptEdit', defaultMessage: 'How should it change?'}) : formatMessage({id: 'fusion.ai.promptWrite', defaultMessage: 'What should it say?'})}
                    aria-label={formatMessage({id: 'fusion.ai.prompt', defaultMessage: 'Instructions for the AI'})}
                    onChange={(e) => setPrompt(e.target.value)}
                />
            </form>
            {message.trim() && ACTIONS.map(([action, label]) => (
                <MenuItem
                    key={action}
                    icon='sparkle'
                    label={formatMessage(label)}
                    onClick={() => rewrite(action)}
                />
            ))}
        </Popover>
    );
}
