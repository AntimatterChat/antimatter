// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {patchTeam} from 'mattermost-redux/actions/teams';
import {getMyChannels} from 'mattermost-redux/selectors/entities/channels';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import Markdown from 'components/markdown';

import Icon from 'fusion/components/icon';
import {Toggle} from 'fusion/modals/settings_controls';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

import {AutolinkPreviewError, PRESETS, newAutolink, parseAutolinks, previewAutolinks, serializeAutolinks} from './autolinks';
import type {Autolink, Autolinks} from './autolinks';

const PREVIEW_DELAY = 400;

type Props = {
    setUnsaved?: (unsaved: boolean) => void;
};

// AutolinksSettings is the Autolinks tab of the team settings: the team's rules turning patterns of its messages
// into links as they're posted (as the community Autolink plugin did), with a message to try them on.
export default function AutolinksSettings({setUnsaved}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const [saved, setSaved] = useState(team?.autolinks || '');
    const [draft, setDraft] = useState<Autolinks>(() => parseAutolinks(team?.autolinks));
    const [sample, setSample] = useState(PRESETS[0].example);
    const [preview, setPreview] = useState('');
    const [error, setError] = useState('');
    const [busy, setBusy] = useState(false);
    const [adding, setAdding] = useState(false);
    const myChannels = useSelector(getMyChannels);
    const channels = useMemo(() => myChannels.filter((c) => c.team_id === team?.id && (c.type === 'O' || c.type === 'P') && c.delete_at === 0).sort((a, b) => a.display_name.localeCompare(b.display_name)), [myChannels, team?.id]);

    const serialized = serializeAutolinks(draft);
    const unsaved = serialized !== saved;
    useEffect(() => {
        setUnsaved?.(unsaved);
    }, [unsaved, setUnsaved]);

    // The preview follows the rules as edited, from the server, which applies them.
    useEffect(() => {
        if (!team) {
            return undefined;
        }
        let live = true;
        const timer = setTimeout(async () => {
            try {
                const text = await previewAutolinks(team.id, draft, sample, true);
                if (live) {
                    setPreview(text);
                    setError('');
                }
            } catch (err) {
                if (live) {
                    setError(err instanceof AutolinkPreviewError ? err.message : String(err));
                }
            }
        }, PREVIEW_DELAY);
        return () => {
            live = false;
            clearTimeout(timer);
        };
    }, [team, serialized, sample]); // eslint-disable-line react-hooks/exhaustive-deps

    if (!team) {
        return null;
    }

    const setLink = (link: Autolink, patch: Partial<Autolink>) => setDraft({...draft, links: draft.links.map((l) => (l.key === link.key ? {...l, ...patch} : l))});
    const toggleChannel = (link: Autolink, id: string) => {
        const current = link.channels || [];
        setLink(link, {channels: current.includes(id) ? current.filter((c) => c !== id) : [...current, id]});
    };

    const save = async () => {
        setBusy(true);
        try {
            // The preview checks the rules and says which one is wrong, which the team patch doesn't.
            await previewAutolinks(team.id, draft, '');
            const result = await dispatch(patchTeam({id: team.id, autolinks: serialized}));
            if ('error' in result && result.error) {
                setError((result.error as {message?: string}).message || formatMessage({id: 'fusion.autolinks.failed', defaultMessage: 'The autolinks couldn\'t be saved.'}));
                return;
            }
            setSaved(serialized);
            toast(formatMessage({id: 'fusion.autolinks.saved', defaultMessage: 'Autolinks saved: they apply to new messages'}));
        } catch (err) {
            setError(err instanceof AutolinkPreviewError ? err.message : String(err));
        } finally {
            setBusy(false);
        }
    };

    return (
        <div className={am('autolinks')}>
            <p className={am('lead')}>
                {formatMessage({id: 'fusion.autolinks.lead', defaultMessage: 'Turn patterns of the messages posted here into links, like issue keys or pull request numbers. A pattern is a regular expression; in the link, $0 is the text it matched and $1, $2… its groups. Code, links and images are left alone.'})}
            </p>
            <Toggle
                title={formatMessage({id: 'fusion.autolinks.onEdit', defaultMessage: 'Link edited messages too'})}
                desc={formatMessage({id: 'fusion.autolinks.onEditDesc', defaultMessage: 'Otherwise, only new messages are linked.'})}
                on={Boolean(draft.on_edit)}
                onChange={(on) => setDraft({...draft, on_edit: on})}
            />
            {draft.links.length === 0 && <p className={am('lead')}>{formatMessage({id: 'fusion.autolinks.none', defaultMessage: 'No autolinks yet.'})}</p>}
            {draft.links.map((link, i) => (
                <div
                    key={link.key}
                    className={am('override-row', 'autolink-row', {off: link.disabled})}
                >
                    <div className={am('override-head')}>
                        <input
                            type='text'
                            className={am('grow')}
                            value={link.name}
                            maxLength={64}
                            placeholder={formatMessage({id: 'fusion.autolinks.name', defaultMessage: 'Name, e.g. Jira issues'})}
                            aria-label={formatMessage({id: 'fusion.autolinks.nameLabel', defaultMessage: 'Name of autolink {n}'}, {n: i + 1})}
                            onChange={(e) => setLink(link, {name: e.target.value})}
                        />
                        <label className={am('check-row')}>
                            <input
                                type='checkbox'
                                checked={!link.disabled}
                                onChange={(e) => setLink(link, {disabled: !e.target.checked})}
                            />
                            {formatMessage({id: 'fusion.autolinks.enabled', defaultMessage: 'On'})}
                        </label>
                        <button
                            type='button'
                            className={am('icon-btn')}
                            aria-label={formatMessage({id: 'fusion.autolinks.remove', defaultMessage: 'Remove the autolink'})}
                            onClick={() => setDraft({...draft, links: draft.links.filter((l) => l.key !== link.key)})}
                        >
                            <Icon
                                name='trash'
                                size='sm'
                            />
                        </button>
                    </div>
                    <div className={am('autolink-fields')}>
                        <label className={am('field')}>
                            <span>{formatMessage({id: 'fusion.autolinks.pattern', defaultMessage: 'Pattern'})}</span>
                            <input
                                type='text'
                                className={am('mono')}
                                value={link.pattern}
                                maxLength={1000}
                                spellCheck={false}
                                placeholder='(PROJ-\d+)'
                                onChange={(e) => setLink(link, {pattern: e.target.value})}
                            />
                        </label>
                        <label className={am('field')}>
                            <span>{formatMessage({id: 'fusion.autolinks.template', defaultMessage: 'Becomes'})}</span>
                            <input
                                type='text'
                                className={am('mono')}
                                value={link.template}
                                maxLength={2000}
                                spellCheck={false}
                                placeholder='[$1](https://issues.example.com/browse/$1)'
                                onChange={(e) => setLink(link, {template: e.target.value})}
                            />
                        </label>
                    </div>
                    <div className={am('autolink-options')}>
                        <label className={am('check-row')}>
                            <input
                                type='checkbox'
                                checked={Boolean(link.word_match)}
                                onChange={(e) => setLink(link, {word_match: e.target.checked})}
                            />
                            {formatMessage({id: 'fusion.autolinks.wordMatch', defaultMessage: 'Whole words only'})}
                        </label>
                        <label className={am('check-row')}>
                            <input
                                type='checkbox'
                                checked={Boolean(link.process_bot_posts)}
                                onChange={(e) => setLink(link, {process_bot_posts: e.target.checked})}
                            />
                            {formatMessage({id: 'fusion.autolinks.bots', defaultMessage: 'Bots\' messages too'})}
                        </label>
                    </div>
                    <div className={am('chip-picks')}>
                        <button
                            type='button'
                            className={am('chip-pick', {on: !link.channels?.length})}
                            onClick={() => setLink(link, {channels: []})}
                        >
                            {formatMessage({id: 'fusion.autolinks.allChannels', defaultMessage: 'All channels'})}
                        </button>
                        {channels.map((c) => (
                            <button
                                key={c.id}
                                type='button'
                                className={am('chip-pick', {on: Boolean(link.channels?.includes(c.id))})}
                                onClick={() => toggleChannel(link, c.id)}
                            >
                                {`#${c.display_name}`}
                            </button>
                        ))}
                    </div>
                </div>
            ))}
            <div className={am('autolink-add')}>
                {adding ? (
                    <div className={am('chip-picks')}>
                        {PRESETS.map((p) => (
                            <button
                                key={p.id}
                                type='button'
                                className={am('chip-pick')}
                                onClick={() => {
                                    setDraft({...draft, links: [...draft.links, newAutolink(p)]});
                                    setSample(p.example);
                                    setAdding(false);
                                }}
                            >
                                {p.name}
                            </button>
                        ))}
                        <button
                            type='button'
                            className={am('chip-pick')}
                            onClick={() => {
                                setDraft({...draft, links: [...draft.links, newAutolink()]});
                                setAdding(false);
                            }}
                        >
                            {formatMessage({id: 'fusion.autolinks.blank', defaultMessage: 'Blank'})}
                        </button>
                    </div>
                ) : (
                    <button
                        type='button'
                        className={am('btn', 'small')}
                        onClick={() => setAdding(true)}
                    >
                        {formatMessage({id: 'fusion.autolinks.add', defaultMessage: 'Add an autolink'})}
                    </button>
                )}
            </div>
            <label className={am('field')}>
                <span>{formatMessage({id: 'fusion.autolinks.try', defaultMessage: 'Try a message'})}</span>
                <textarea
                    rows={2}
                    value={sample}
                    maxLength={4000}
                    onChange={(e) => setSample(e.target.value)}
                />
            </label>
            <div
                className={am('autolink-preview')}
                aria-live='polite'
            >
                <Markdown message={preview || sample}/>
            </div>
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
                    disabled={!unsaved || busy}
                    onClick={() => {
                        setDraft(parseAutolinks(saved));
                        setError('');
                    }}
                >
                    {formatMessage({id: 'fusion.autolinks.reset', defaultMessage: 'Undo changes'})}
                </button>
                <button
                    type='button'
                    className={am('btn', 'primary')}
                    disabled={!unsaved || busy}
                    onClick={save}
                >
                    {formatMessage({id: 'fusion.autolinks.save', defaultMessage: 'Save'})}
                </button>
            </div>
        </div>
    );
}
