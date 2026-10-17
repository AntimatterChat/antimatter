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
        desc: defineMessage({id: 'fusion.layout.hybridDesc', defaultMessage: 'Curated categories for everyone, an open Community one where members file their channels, and a Moderators role. Only the community\'s admins archive channels.'}),
    },
    {
        preset: 'community',
        title: defineMessage({id: 'fusion.layout.community', defaultMessage: 'Community'}),
        desc: defineMessage({id: 'fusion.layout.communityDesc', defaultMessage: 'Categories the admins keep, and a Moderators role. Only the community\'s admins archive channels. To stop members creating channels, change the community\'s permissions in the System Console.'}),
    },
    {
        preset: 'workplace',
        title: defineMessage({id: 'fusion.layout.workplace', defaultMessage: 'Workplace'}),
        desc: defineMessage({id: 'fusion.layout.workplaceDesc', defaultMessage: 'Like Mattermost: everyone keeps their own categories, and archives the channels they created. Removes the categories for everyone; roles stay.'}),
    },
];

// TeamLayoutDialog offers to set a new team up from a preset, the first time its admin opens it.
export default function TeamLayoutDialog({onClose, first = false}: {onClose: () => void; first?: boolean}) {
    const {formatMessage} = useIntl();
    const team = useSelector(getCurrentTeam);
    return (
        <Dialog
            label={formatMessage({id: 'fusion.layout.title', defaultMessage: 'Community layout'})}
            onClose={onClose}
            small={false}
            className={am('wide')}
        >
            <div className={am('pane', 'roles-pane')}>
                <h2>{first ? formatMessage({id: 'fusion.layout.firstTitle', defaultMessage: 'Set up {team}'}, {team: team?.display_name}) : formatMessage({id: 'fusion.layout.title', defaultMessage: 'Community layout'})}</h2>
                <TeamLayoutSettings
                    first={first}
                    onClose={onClose}
                />
            </div>
        </Dialog>
    );
}

// TeamLayoutSettings sets a team up from a preset or a template, and exports its layout as a template: the Community
// layout tab of the community settings, and the first-time dialog's content. Close (Not now, the first time) closes
// them; applying a preset closes the first-time dialog.
export function TeamLayoutSettings({onClose, first = false}: {onClose: () => void; first?: boolean}) {
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
        let failed = 0;
        try {
            const result = await dispatch(applyTemplate(team.id, template));
            failed = 'data' in result && result.data ? result.data.failed : 1;
        } catch {
            failed = 1;
        } finally {
            setBusy(false);
        }
        if (failed) {
            toast(formatMessage({id: 'fusion.layout.partly', defaultMessage: '{team} is set up, but {count, plural, one {# step} other {# steps}} failed'}, {team: team.display_name, count: failed}));
        } else {
            toast(formatMessage({id: 'fusion.layout.applied', defaultMessage: '{team} is set up'}, {team: team.display_name}));
        }
        if (first) {
            onClose();
        }
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
            toast(formatMessage({id: 'fusion.layout.notTemplate', defaultMessage: 'That file isn\'t a community template'}));
            return;
        }
        await apply(template);
    };

    return (
        <>
            <p className={am('lead')}>{formatMessage({id: 'fusion.layout.lead', defaultMessage: 'Start from a preset, or from a template exported from another community. Applying it only removes the community categories it doesn\'t include.'})}</p>
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
                {!first && (
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.teamSettings.closeButton', defaultMessage: 'Close'})}
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
        </>
    );
}
