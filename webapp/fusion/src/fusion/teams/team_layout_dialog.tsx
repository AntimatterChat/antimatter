// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useRef, useState} from 'react';
import {defineMessage, useIntl} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {Dialog} from 'fusion/components/layer';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';

import {applyTemplate, exportTemplate, parseTemplate, PRESETS} from './templates';
import type {Preset, TeamTemplate} from './templates';

const PRESET_TEXT: Array<{preset: Preset; title: MessageDescriptor; desc: MessageDescriptor}> = [
    {
        preset: 'hybrid',
        title: defineMessage({id: 'fusion.layout.hybrid', defaultMessage: 'Hybrid (recommended)'}),
        desc: defineMessage({id: 'fusion.layout.hybridDesc', defaultMessage: 'Curated categories for everyone, an open Community one where members file their channels, and a Moderators role.'}),
    },
    {
        preset: 'community',
        title: defineMessage({id: 'fusion.layout.community', defaultMessage: 'Community'}),
        desc: defineMessage({id: 'fusion.layout.communityDesc', defaultMessage: 'Like Discord: categories the admins keep, and a Moderators role. To stop members creating channels, change the team\'s permissions in the System Console.'}),
    },
    {
        preset: 'workplace',
        title: defineMessage({id: 'fusion.layout.workplace', defaultMessage: 'Workplace'}),
        desc: defineMessage({id: 'fusion.layout.workplaceDesc', defaultMessage: 'Like Mattermost: everyone keeps their own categories. Removes the categories for everyone; roles stay.'}),
    },
];

// TeamLayoutDialog sets a team up from a preset or a template, and exports its layout as a template.
export default function TeamLayoutDialog({onClose, first = false}: {onClose: () => void; first?: boolean}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const [chosen, setChosen] = useState<Preset>('hybrid');
    const [busy, setBusy] = useState(false);
    const fileRef = useRef<HTMLInputElement>(null);

    const apply = async (template: TeamTemplate) => {
        if (!team) {
            return;
        }
        setBusy(true);
        await dispatch(applyTemplate(team.id, template));
        setBusy(false);
        toast(formatMessage({id: 'fusion.layout.applied', defaultMessage: '{team} is set up'}, {team: team.display_name}));
        onClose();
    };

    const download = async () => {
        if (!team) {
            return;
        }
        const result = await dispatch(exportTemplate(team.id));
        if (!('data' in result) || !result.data) {
            return;
        }
        const url = URL.createObjectURL(new Blob([JSON.stringify(result.data, null, 2)], {type: 'application/json'}));
        const link = document.createElement('a');
        link.href = url;
        link.download = `${team.name}-template.json`;
        link.click();
        URL.revokeObjectURL(url);
    };

    const load = async (file: File | undefined) => {
        if (!file) {
            return;
        }
        const template = parseTemplate(await file.text());
        if (!template) {
            toast(formatMessage({id: 'fusion.layout.notTemplate', defaultMessage: 'That file isn\'t a team template'}));
            return;
        }
        apply(template);
    };

    return (
        <Dialog
            label={formatMessage({id: 'fusion.layout.title', defaultMessage: 'Team layout'})}
            onClose={onClose}
            small={false}
            className={am('wide')}
        >
            <div className={am('pane', 'roles-pane')}>
                <h2>{first ? formatMessage({id: 'fusion.layout.firstTitle', defaultMessage: 'Set up {team}'}, {team: team?.display_name}) : formatMessage({id: 'fusion.layout.title', defaultMessage: 'Team layout'})}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.layout.lead', defaultMessage: 'Start from a preset, or a template exported from another team. Nothing is deleted but categories for everyone the layout doesn\'t have.'})}</p>
                <div
                    className={am('presets')}
                    role='radiogroup'
                >
                    {PRESET_TEXT.map((p) => (
                        <button
                            key={p.preset}
                            type='button'
                            role='radio'
                            aria-checked={chosen === p.preset}
                            className={am('preset', {on: chosen === p.preset})}
                            onClick={() => setChosen(p.preset)}
                        >
                            <b>{formatMessage(p.title)}</b>
                            <span>{formatMessage(p.desc)}</span>
                        </button>
                    ))}
                </div>
                <input
                    ref={fileRef}
                    type='file'
                    accept='application/json,.json'
                    style={{display: 'none'}}
                    onChange={(e) => load(e.target.files?.[0])}
                />
                <div className={am('modal-actions')}>
                    {!first && (
                        <>
                            <button
                                type='button'
                                className={am('btn')}
                                onClick={download}
                            >
                                {formatMessage({id: 'fusion.layout.export', defaultMessage: 'Export as a template'})}
                            </button>
                            <button
                                type='button'
                                className={am('btn')}
                                disabled={busy}
                                onClick={() => fileRef.current?.click()}
                            >
                                {formatMessage({id: 'fusion.layout.import', defaultMessage: 'Apply a template…'})}
                            </button>
                        </>
                    )}
                    {first && (
                        <button
                            type='button'
                            className={am('btn')}
                            onClick={onClose}
                        >
                            {formatMessage({id: 'fusion.layout.later', defaultMessage: 'Not now'})}
                        </button>
                    )}
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        disabled={busy}
                        onClick={() => apply(PRESETS[chosen])}
                    >
                        {formatMessage({id: 'fusion.layout.apply', defaultMessage: 'Apply'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
