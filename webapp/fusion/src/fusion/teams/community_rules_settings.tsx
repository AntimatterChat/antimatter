// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {CommunityRules, CommunityRulesStats} from '@mattermost/types/community_rules';

import {Client4} from 'mattermost-redux/client';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import Markdown from 'components/markdown';

import {Toggle} from 'fusion/modals/settings_controls';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

import {saveCommunityRules} from './community_rules';

// The bound of the rules' text, as the server's (model.CommunityRulesMaxRunes).
export const COMMUNITY_RULES_MAX_LENGTH = 16000;

type Saved = Pick<CommunityRules, 'enabled' | 'text' | 'version'>;

const NONE: Saved = {enabled: false, text: '', version: 0};

// CommunityRulesSettings is the Rules tab of the community settings, for its admins: rules members agree to before
// they post, turned on or off, written in Markdown with a preview, and how many members agreed. Saving can ask
// everyone to agree again; otherwise those who agreed stay agreed (to fix a typo).
export default function CommunityRulesSettings({setUnsaved}: {setUnsaved?: (unsaved: boolean) => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const teamId = team?.id || '';
    const [saved, setSaved] = useState<Saved | null>(null);
    const [enabled, setEnabled] = useState(false);
    const [text, setText] = useState('');
    const [askAgain, setAskAgain] = useState(false);
    const [previewing, setPreviewing] = useState(false);
    const [stats, setStats] = useState<CommunityRulesStats | null>(null);
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);

    const loadStats = useCallback(async () => {
        try {
            setStats(await Client4.getCommunityRulesStats(teamId));
        } catch {
            setStats(null);
        }
    }, [teamId]);

    useEffect(() => {
        if (!teamId) {
            return undefined;
        }
        let live = true;
        Client4.getCommunityRules(teamId).then((rules) => {
            if (live) {
                setSaved({enabled: rules.enabled, text: rules.text, version: rules.version});
                setEnabled(rules.enabled);
                setText(rules.text);
            }
        }).catch(() => {
            if (live) {
                setSaved(NONE);
            }
        });
        loadStats();
        return () => {
            live = false;
        };
    }, [teamId, loadStats]);

    const unsaved = saved !== null && (enabled !== saved.enabled || text.trim() !== saved.text);
    useEffect(() => {
        setUnsaved?.(unsaved || askAgain);
    }, [unsaved, askAgain, setUnsaved]);
    useEffect(() => () => setUnsaved?.(false), [setUnsaved]);

    if (!team || !saved) {
        return <p className={am('lead')}>{formatMessage({id: 'fusion.communityRules.loading', defaultMessage: 'Loading the rules…'})}</p>;
    }

    const hadRules = saved.version > 0;
    const save = async () => {
        if (enabled && !text.trim()) {
            setError(formatMessage({id: 'fusion.communityRules.empty', defaultMessage: 'Write the rules before turning them on.'}));
            return;
        }
        setBusy(true);
        setError('');
        const result = await dispatch(saveCommunityRules(team.id, {enabled, text: text.trim(), ask_again: hadRules && askAgain}));
        setBusy(false);
        if ('error' in result && result.error) {
            setError((result.error as {message?: string}).message || formatMessage({id: 'fusion.communityRules.saveFailed', defaultMessage: 'The rules couldn\'t be saved.'}));
            return;
        }
        const rules = result.data as CommunityRules;
        setSaved({enabled: rules.enabled, text: rules.text, version: rules.version});
        setText(rules.text);
        setAskAgain(false);
        loadStats();
        if (!rules.enabled) {
            toast(formatMessage({id: 'fusion.communityRules.savedOff', defaultMessage: 'Rules saved: they\'re off, everyone can post'}));
        } else if (rules.version !== saved.version || !saved.enabled) {
            toast(formatMessage({id: 'fusion.communityRules.savedAsk', defaultMessage: 'Rules saved: members agree to them before they post'}));
        } else {
            toast(formatMessage({id: 'fusion.communityRules.saved', defaultMessage: 'Rules saved'}));
        }
    };

    return (
        <div className={am('community-rules-settings')}>
            <p className={am('lead')}>
                {formatMessage({id: 'fusion.communityRules.settingsLead', defaultMessage: 'Rules members read and agree to before they post in this community\'s channels. They\'re shown when someone joins, and in place of the message box until they agree. Admins and bots don\'t have to agree; direct messages aren\'t affected.'})}
            </p>
            <Toggle
                title={formatMessage({id: 'fusion.communityRules.enable', defaultMessage: 'Members must agree to the rules'})}
                desc={formatMessage({id: 'fusion.communityRules.enableDesc', defaultMessage: 'Until they agree, they can read but not post.'})}
                on={enabled}
                onChange={setEnabled}
            />
            <div
                className={am('seg', 'community-rules-tabs')}
                role='tablist'
            >
                <button
                    type='button'
                    role='tab'
                    aria-selected={!previewing}
                    className={previewing ? '' : am('on')}
                    onClick={() => setPreviewing(false)}
                >
                    {formatMessage({id: 'fusion.communityRules.write', defaultMessage: 'Write'})}
                </button>
                <button
                    type='button'
                    role='tab'
                    aria-selected={previewing}
                    className={previewing ? am('on') : ''}
                    onClick={() => setPreviewing(true)}
                >
                    {formatMessage({id: 'fusion.communityRules.preview', defaultMessage: 'Preview'})}
                </button>
            </div>
            {previewing ? (
                <div
                    className={am('community-rules-text', 'community-rules-preview')}
                    role='tabpanel'
                >
                    {text.trim() ? <Markdown message={text}/> : formatMessage({id: 'fusion.communityRules.previewEmpty', defaultMessage: 'Nothing to preview yet.'})}
                </div>
            ) : (
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.communityRules.text', defaultMessage: 'Rules (Markdown)'})}</span>
                    <textarea
                        className={am('community-rules-input')}
                        rows={10}
                        value={text}
                        maxLength={COMMUNITY_RULES_MAX_LENGTH}
                        placeholder={formatMessage({id: 'fusion.communityRules.placeholder', defaultMessage: '1. Be kind and respectful.\n2. No spam or self-promotion.\n3. Keep each channel on its topic.'})}
                        onChange={(e) => setText(e.target.value)}
                    />
                </label>
            )}
            {hadRules ? (
                <label className={am('check-row')}>
                    <input
                        type='checkbox'
                        checked={askAgain}
                        onChange={(e) => setAskAgain(e.target.checked)}
                    />
                    <span>
                        <b>{formatMessage({id: 'fusion.communityRules.askAgain', defaultMessage: 'Ask everyone to agree again'})}</b>
                        {' '}
                        {formatMessage({id: 'fusion.communityRules.askAgainDesc', defaultMessage: 'For a change that matters: members who agreed must agree to the new rules before they post again. Leave it off to fix a typo.'})}
                    </span>
                </label>
            ) : (
                <p className={am('community-rules-hint')}>
                    {formatMessage({id: 'fusion.communityRules.firstTime', defaultMessage: 'Once the rules are on, every member agrees to them before they post.'})}
                </p>
            )}
            {saved.enabled && stats && (
                <p
                    className={am('community-rules-stats')}
                    aria-live='polite'
                >
                    {formatMessage({id: 'fusion.communityRules.stats', defaultMessage: '{accepted, number} of {members, plural, one {# member} other {# members}} agreed to the current rules.'}, {accepted: stats.accepted_count, members: stats.member_count})}
                </p>
            )}
            {error && (
                <div
                    className={am('field-error')}
                    role='alert'
                >
                    {error}
                </div>
            )}
            <div className={am('modal-actions')}>
                <button
                    type='button'
                    className={am('btn')}
                    disabled={(!unsaved && !askAgain) || busy}
                    onClick={() => {
                        setEnabled(saved.enabled);
                        setText(saved.text);
                        setAskAgain(false);
                        setError('');
                    }}
                >
                    {formatMessage({id: 'fusion.communityRules.reset', defaultMessage: 'Undo changes'})}
                </button>
                <button
                    type='button'
                    className={am('btn', 'primary')}
                    disabled={(!unsaved && !askAgain) || busy}
                    onClick={save}
                >
                    {formatMessage({id: 'fusion.communityRules.save', defaultMessage: 'Save'})}
                </button>
            </div>
        </div>
    );
}
