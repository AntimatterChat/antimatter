// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {getAgents as fetchAgents} from 'mattermost-redux/actions/agents';
import {savePreferences} from 'mattermost-redux/actions/preferences';
import {Client4} from 'mattermost-redux/client';
import {getAgents} from 'mattermost-redux/selectors/entities/agents';
import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {RewriteAction} from 'components/advanced_text_editor/rewrite_action';
import {useSelectedAgent} from 'components/common/agents';

import type {IconName} from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {MenuHeading, MenuItem, MenuSeparator} from 'fusion/components/menu';
import {am} from 'fusion/utils/class_names';
import PluggableErrorBoundary from 'plugins/pluggable/error_boundary';

import type {GlobalState} from 'types/store';
import type {AIActionMenuItemComponent, AIActionMenuItemProps} from 'types/store/plugins';

// The mockup's AI actions: Mattermost's rewrite actions, and instructions for the others.
type AiAction = {key: string; icon: IconName; label: {id: string; defaultMessage: string}; action: RewriteAction; prompt?: string};

const ACTIONS: AiAction[] = [
    {key: 'improve', icon: 'sparkle', label: {id: 'fusion.ai.improve', defaultMessage: 'Improve writing'}, action: RewriteAction.IMPROVE_WRITING},
    {key: 'fix', icon: 'check', label: {id: 'fusion.ai.spelling', defaultMessage: 'Fix spelling & grammar'}, action: RewriteAction.FIX_SPELLING},
    {key: 'shorter', icon: 'ul', label: {id: 'fusion.ai.shorten', defaultMessage: 'Make it shorter'}, action: RewriteAction.SHORTEN},
    {key: 'formal', icon: 'type', label: {id: 'fusion.ai.formal', defaultMessage: 'Make it more formal'}, action: RewriteAction.CUSTOM, prompt: 'Rewrite this message in a more formal, professional tone. Keep its meaning, its language and its formatting.'},
];

// The language last translated into, offered again.
const CATEGORY_FUSION_AI = 'fusion_ai';
const NAME_LANGUAGE = 'translate_to';

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

    pluginItems: AIActionMenuItemComponent[];
    pluginProps: AIActionMenuItemProps;
    message: string;
    rootId: string;
    onRewritten: (message: string, original: string) => void;
    onError: (message: string) => void;
    onBusy: (busy: boolean) => void;
    onClose: () => void;
};

// AiRewriteMenu has an AI agent rewrite the message being written, as the mockup's AI menu: improve, fix, shorten,
// make more formal, translate; then plugins' AI items (the Agents app's custom prompts and their management).
export default function AiRewriteMenu({anchor, pluginItems, pluginProps, message, rootId, onRewritten, onError, onBusy, onClose}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const agents = useSelector(getAgents);
    const [agentId] = useSelectedAgent(agents);
    const userId = useSelector(getCurrentUserId);
    const lastLanguage = useSelector((state: GlobalState) => getPreference(state, CATEGORY_FUSION_AI, NAME_LANGUAGE, ''));
    const [translating, setTranslating] = useState(false);
    const [language, setLanguage] = useState(lastLanguage);
    const hasText = Boolean(message.trim());

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
    const translate = () => {
        const to = language.trim();
        if (!to) {
            return;
        }
        dispatch(savePreferences(userId, [{user_id: userId, category: CATEGORY_FUSION_AI, name: NAME_LANGUAGE, value: to}]));
        rewrite(RewriteAction.CUSTOM, `Translate this message into ${to}. Keep its formatting, mentions, links and emoji; only give the translation.`);
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
            {!hasText && <p className={am('ai-hint')}>{formatMessage({id: 'fusion.ai.writeFirst', defaultMessage: 'Write your message first.'})}</p>}
            {ACTIONS.map((a) => (
                <MenuItem
                    key={a.key}
                    icon={a.icon}
                    label={formatMessage(a.label)}
                    disabled={!hasText}
                    onClick={() => rewrite(a.action, a.prompt)}
                />
            ))}
            {!translating && (
                <MenuItem
                    icon='globe'
                    label={formatMessage({id: 'fusion.ai.translate', defaultMessage: 'Translate…'})}
                    disabled={!hasText}
                    onClick={() => setTranslating(true)}
                />
            )}
            {translating && (
                <form
                    className={am('ai-prompt')}
                    onSubmit={(e) => {
                        e.preventDefault();
                        translate();
                    }}
                >
                    <input
                        autoFocus={true}
                        value={language}
                        placeholder={formatMessage({id: 'fusion.ai.language', defaultMessage: 'Into which language? e.g. French'})}
                        aria-label={formatMessage({id: 'fusion.ai.languageLabel', defaultMessage: 'Language to translate into'})}
                        onChange={(e) => setLanguage(e.target.value)}
                    />
                </form>
            )}
            {pluginItems.length > 0 && <MenuSeparator/>}
            {[...pluginItems].sort((a, b) => a.sortOrder - b.sortOrder).map((item) => {
                const Component = item.component;
                if (Component) {
                    return (
                        <PluggableErrorBoundary
                            key={item.id}
                            pluginId={item.pluginId}
                        >
                            <Component {...pluginProps}/>
                        </PluggableErrorBoundary>
                    );
                }
                return (
                    <button
                        key={item.id}
                        type='button'
                        role='menuitem'
                        onClick={() => {
                            onClose();
                            item.action?.(pluginProps);
                        }}
                    >
                        <span className={am('plugin-ic')}>{item.icon}</span>
                        {item.text}
                    </button>
                );
            })}
        </Popover>
    );
}
