// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {defineMessage, useIntl} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';
import {useSelector} from 'react-redux';

import type {FieldVisibility} from '@mattermost/types/properties';
import type {CommunityAttributeField, CommunityAttributeFieldPatch} from '@mattermost/types/properties_user';

import {getTeam} from 'mattermost-redux/selectors/entities/teams';

import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import {Choice, Toggle} from 'fusion/modals/settings_controls';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

import {createCommunityAttribute, deleteCommunityAttribute, patchCommunityAttribute, useCommunityAttributes} from './community_attributes';

// The kinds of attributes offered: the field types, with text split by what it holds.
type Kind = 'text' | 'email' | 'url' | 'phone' | 'select' | 'multiselect' | 'date' | 'user' | 'multiuser';

export const KINDS: Array<[Kind, MessageDescriptor]> = [
    ['text', defineMessage({id: 'fusion.communityAttributes.kind.text', defaultMessage: 'Text'})],
    ['select', defineMessage({id: 'fusion.communityAttributes.kind.select', defaultMessage: 'Choice'})],
    ['multiselect', defineMessage({id: 'fusion.communityAttributes.kind.multiselect', defaultMessage: 'Several choices'})],
    ['date', defineMessage({id: 'fusion.communityAttributes.kind.date', defaultMessage: 'Date'})],
    ['url', defineMessage({id: 'fusion.communityAttributes.kind.url', defaultMessage: 'Link'})],
    ['email', defineMessage({id: 'fusion.communityAttributes.kind.email', defaultMessage: 'Email'})],
    ['phone', defineMessage({id: 'fusion.communityAttributes.kind.phone', defaultMessage: 'Phone number'})],
    ['user', defineMessage({id: 'fusion.communityAttributes.kind.user', defaultMessage: 'Person'})],
    ['multiuser', defineMessage({id: 'fusion.communityAttributes.kind.multiuser', defaultMessage: 'People'})],
];

function kindOf(field: CommunityAttributeField): Kind {
    if (field.type === 'text') {
        return (field.attrs?.value_type as Kind) || 'text';
    }
    return field.type as Kind;
}

type Option = {id?: string; name: string; color?: string};

type Draft = {
    name: string;
    kind: Kind;
    options: Option[];
    managed: boolean;
    visibility: FieldVisibility;
    memberList: boolean;
};

function draftOf(field?: CommunityAttributeField): Draft {
    return {
        name: field?.name || '',
        kind: field ? kindOf(field) : 'text',
        options: field?.attrs?.options?.map((o) => ({id: o.id, name: o.name, color: o.color})) || [{name: ''}],
        managed: field?.attrs?.managed === 'admin',
        visibility: field?.attrs?.visibility || 'when_set',
        memberList: Boolean(field?.attrs?.member_list),
    };
}

function patchOf(draft: Draft): CommunityAttributeFieldPatch {
    const select = draft.kind === 'select' || draft.kind === 'multiselect';
    const text = draft.kind === 'email' || draft.kind === 'url' || draft.kind === 'phone';
    return {
        name: draft.name.trim(),
        type: text ? 'text' : draft.kind,
        attrs: {
            managed: draft.managed ? 'admin' : '',
            visibility: draft.visibility,
            member_list: draft.memberList,
            ...(draft.kind === 'text' || text ? {value_type: text ? draft.kind as 'email' | 'url' | 'phone' : ''} : {}),
            ...(select ? {options: draft.options.filter((o) => o.name.trim()).map((o) => ({id: o.id || '', name: o.name.trim(), color: o.color || ''}))} : {}),
        },
    } as CommunityAttributeFieldPatch;
}

type Props = {
    teamId: string;
    toast?: (text: string) => void;
    onHide?: () => void;
};

// CommunityAttributesDialog is where a community's admins define its members' attributes (who they work for, their
// site…): shown on profiles within the community, set by the admins or by each member.
export default function CommunityAttributesDialog({teamId, toast, onHide}: Props) {
    const {formatMessage} = useIntl();
    const team = useSelector((state: GlobalState) => getTeam(state, teamId));
    const fields = useCommunityAttributes(teamId) || [];
    const [editing, setEditing] = useState<string | 'new' | null>(null);
    const [deleting, setDeleting] = useState<string | null>(null);
    const [error, setError] = useState('');
    const close = () => onHide?.();

    const attempt = async (action: () => Promise<unknown>) => {
        setError('');
        try {
            await action();
            return true;
        } catch (err) {
            setError((err as {message?: string}).message || formatMessage({id: 'fusion.communityAttributes.failed', defaultMessage: 'The attribute couldn\'t be saved.'}));
            return false;
        }
    };

    const save = async (draft: Draft, field?: CommunityAttributeField) => {
        const patch = patchOf(draft);
        const done = await attempt(() => (field ? patchCommunityAttribute(teamId, field.id, {name: patch.name, attrs: patch.attrs}) : createCommunityAttribute(teamId, patch)));
        if (done) {
            setEditing(null);
        }
    };

    // Moving an attribute renumbers them all in their new order.
    const move = (at: number, by: -1 | 1) => attempt(async () => {
        const order = [...fields];
        [order[at], order[at + by]] = [order[at + by], order[at]];
        for (const [i, field] of order.entries()) {
            if (field.attrs?.sort_order !== i) {
                // eslint-disable-next-line no-await-in-loop
                await patchCommunityAttribute(teamId, field.id, {attrs: {sort_order: i}});
            }
        }
    });

    const remove = async (field: CommunityAttributeField) => {
        if (await attempt(() => deleteCommunityAttribute(teamId, field.id))) {
            setDeleting(null);
            toast?.(formatMessage({id: 'fusion.communityAttributes.deleted', defaultMessage: 'Deleted the attribute {name}'}, {name: field.name}));
        }
    };

    const kindLabel = (field: CommunityAttributeField) => formatMessage(KINDS.find(([k]) => k === kindOf(field))?.[1] || KINDS[0][1]);
    const title = formatMessage({id: 'fusion.communityAttributes.title', defaultMessage: 'Member attributes'});
    const edited = fields.find((f) => f.id === editing);

    return (
        <Dialog
            label={title}
            onClose={close}
            small={false}
            className={am('wide')}
        >
            <div className={am('pane', 'roles-pane')}>
                <h2>{title}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.communityAttributes.lead', defaultMessage: 'Details about the members of {community}, such as who they work for or where: shown on their profiles in this community only. Set them from a member\'s menu, "Edit attributes…".'}, {community: team?.display_name || ''})}</p>
                <div className={am('role-list')}>
                    {fields.map((field, i) => (
                        <div
                            key={field.id}
                            className={am('role-row')}
                        >
                            <Icon
                                name='tag'
                                size='sm'
                            />
                            <div className={am('grow')}>
                                <b>{field.name}</b>
                                <span>
                                    {kindLabel(field)}
                                    {field.attrs?.managed === 'admin' ? ` · ${formatMessage({id: 'fusion.communityAttributes.byAdmins', defaultMessage: 'set by admins'})}` : ` · ${formatMessage({id: 'fusion.communityAttributes.byMembers', defaultMessage: 'set by members'})}`}
                                    {field.attrs?.visibility === 'admin' && ` · ${formatMessage({id: 'fusion.communityAttributes.adminsOnly', defaultMessage: 'admins only'})}`}
                                    {field.attrs?.visibility === 'hidden' && ` · ${formatMessage({id: 'fusion.communityAttributes.hidden', defaultMessage: 'hidden'})}`}
                                    {field.attrs?.member_list && ` · ${formatMessage({id: 'fusion.communityAttributes.inList', defaultMessage: 'in the member list'})}`}
                                </span>
                            </div>
                            {deleting === field.id ? (
                                <>
                                    <span className={am('attr-confirm')}>{formatMessage({id: 'fusion.communityAttributes.deleteConfirm', defaultMessage: 'Delete it and everyone\'s values?'})}</span>
                                    <button
                                        type='button'
                                        className={am('btn')}
                                        onClick={() => setDeleting(null)}
                                    >
                                        {formatMessage({id: 'fusion.communityAttributes.cancel', defaultMessage: 'Cancel'})}
                                    </button>
                                    <button
                                        type='button'
                                        className={am('btn', 'danger')}
                                        onClick={() => remove(field)}
                                    >
                                        {formatMessage({id: 'fusion.communityAttributes.deleteIt', defaultMessage: 'Delete'})}
                                    </button>
                                </>
                            ) : (
                                <>
                                    <button
                                        type='button'
                                        className={am('icon-btn')}
                                        title={formatMessage({id: 'fusion.communityAttributes.up', defaultMessage: 'Move up'})}
                                        aria-label={formatMessage({id: 'fusion.communityAttributes.up', defaultMessage: 'Move up'})}
                                        disabled={i === 0}
                                        onClick={() => move(i, -1)}
                                    >
                                        <Icon
                                            name='chev'
                                            size='sm'
                                            className={am('flip')}
                                        />
                                    </button>
                                    <button
                                        type='button'
                                        className={am('icon-btn')}
                                        title={formatMessage({id: 'fusion.communityAttributes.down', defaultMessage: 'Move down'})}
                                        aria-label={formatMessage({id: 'fusion.communityAttributes.down', defaultMessage: 'Move down'})}
                                        disabled={i === fields.length - 1}
                                        onClick={() => move(i, 1)}
                                    >
                                        <Icon
                                            name='chev'
                                            size='sm'
                                        />
                                    </button>
                                    <button
                                        type='button'
                                        className={am('icon-btn')}
                                        title={formatMessage({id: 'fusion.communityAttributes.edit', defaultMessage: 'Edit the attribute'})}
                                        aria-label={formatMessage({id: 'fusion.communityAttributes.edit', defaultMessage: 'Edit the attribute'})}
                                        onClick={() => setEditing(editing === field.id ? null : field.id)}
                                    >
                                        <Icon
                                            name='pen'
                                            size='sm'
                                        />
                                    </button>
                                    <button
                                        type='button'
                                        className={am('icon-btn', 'danger')}
                                        title={formatMessage({id: 'fusion.communityAttributes.delete', defaultMessage: 'Delete the attribute'})}
                                        aria-label={formatMessage({id: 'fusion.communityAttributes.delete', defaultMessage: 'Delete the attribute'})}
                                        onClick={() => setDeleting(field.id)}
                                    >
                                        <Icon
                                            name='trash'
                                            size='sm'
                                        />
                                    </button>
                                </>
                            )}
                        </div>
                    ))}
                    {!fields.length && <p className={am('lead')}>{formatMessage({id: 'fusion.communityAttributes.none', defaultMessage: 'No attributes yet. Add one, such as "Company" or "Location".'})}</p>}
                </div>
                {editing && (
                    <AttributeEditor
                        key={editing}
                        field={edited}
                        onSave={(draft) => save(draft, edited)}
                        onCancel={() => setEditing(null)}
                    />
                )}
                {error && (
                    <p
                        className={am('field-error')}
                        role='alert'
                    >
                        {error}
                    </p>
                )}
                <div className={am('modal-actions')}>
                    {!editing && fields.length < 20 && (
                        <button
                            type='button'
                            className={am('btn')}
                            onClick={() => setEditing('new')}
                        >
                            <Icon
                                name='plus'
                                size='sm'
                            />
                            {formatMessage({id: 'fusion.communityAttributes.create', defaultMessage: 'Add attribute'})}
                        </button>
                    )}
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        onClick={close}
                    >
                        {formatMessage({id: 'fusion.communityAttributes.done', defaultMessage: 'Done'})}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}

type EditorProps = {
    field?: CommunityAttributeField;
    onSave: (draft: Draft) => void;
    onCancel: () => void;
};

// AttributeEditor edits an attribute: its name, kind (fixed once it exists, as its values would not fit another),
// choices, who sets it and who sees it.
function AttributeEditor({field, onSave, onCancel}: EditorProps) {
    const {formatMessage} = useIntl();
    const [draft, setDraft] = useState<Draft>(() => draftOf(field));
    const set = (patch: Partial<Draft>) => setDraft((d) => ({...d, ...patch}));
    const select = draft.kind === 'select' || draft.kind === 'multiselect';
    const setOption = (i: number, name: string) => set({options: draft.options.map((o, j) => (j === i ? {...o, name} : o))});
    const valid = draft.name.trim() && (!select || draft.options.some((o) => o.name.trim()));

    return (
        <div className={am('role-editor')}>
            <label className={am('field')}>
                <span>{formatMessage({id: 'fusion.communityAttributes.name', defaultMessage: 'Name'})}</span>
                <input
                    type='text'
                    value={draft.name}
                    maxLength={64}
                    placeholder={formatMessage({id: 'fusion.communityAttributes.namePlaceholder', defaultMessage: 'e.g. Company'})}
                    onChange={(e) => set({name: e.target.value})}
                />
            </label>
            <label className={am('field')}>
                <span>{formatMessage({id: 'fusion.communityAttributes.kind', defaultMessage: 'Kind'})}</span>
                <select
                    className={am('select')}
                    value={draft.kind}
                    disabled={Boolean(field)}
                    onChange={(e) => set({kind: e.target.value as Kind})}
                >
                    {KINDS.map(([kind, label]) => (
                        <option
                            key={kind}
                            value={kind}
                        >
                            {formatMessage(label)}
                        </option>
                    ))}
                </select>
            </label>
            {select && (
                <div className={am('field', 'attr-options')}>
                    <span>{formatMessage({id: 'fusion.communityAttributes.options', defaultMessage: 'Choices'})}</span>
                    {draft.options.map((option, i) => (
                        <div
                            key={option.id || `new-${i}`}
                            className={am('attr-option')}
                        >
                            <input
                                type='text'
                                value={option.name}
                                maxLength={128}
                                aria-label={formatMessage({id: 'fusion.communityAttributes.optionLabel', defaultMessage: 'Choice {n}'}, {n: i + 1})}
                                onChange={(e) => setOption(i, e.target.value)}
                            />
                            <button
                                type='button'
                                className={am('icon-btn')}
                                aria-label={formatMessage({id: 'fusion.communityAttributes.removeOption', defaultMessage: 'Remove the choice'})}
                                disabled={draft.options.length === 1}
                                onClick={() => set({options: draft.options.filter((_, j) => j !== i)})}
                            >
                                <Icon
                                    name='x'
                                    size='sm'
                                />
                            </button>
                        </div>
                    ))}
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={() => set({options: [...draft.options, {name: ''}]})}
                    >
                        <Icon
                            name='plus'
                            size='sm'
                        />
                        {formatMessage({id: 'fusion.communityAttributes.addOption', defaultMessage: 'Add a choice'})}
                    </button>
                </div>
            )}
            <Choice<'members' | 'admins'>
                title={formatMessage({id: 'fusion.communityAttributes.setBy', defaultMessage: 'Who sets it'})}
                value={draft.managed ? 'admins' : 'members'}
                options={[
                    ['members', formatMessage({id: 'fusion.communityAttributes.setByMembers', defaultMessage: 'Each member'})],
                    ['admins', formatMessage({id: 'fusion.communityAttributes.setByAdmins', defaultMessage: 'Admins'})],
                ]}
                onChange={(value) => set({managed: value === 'admins'})}
            />
            <Choice<FieldVisibility>
                title={formatMessage({id: 'fusion.communityAttributes.shownTo', defaultMessage: 'Who sees it'})}
                desc={formatMessage({id: 'fusion.communityAttributes.shownToDesc', defaultMessage: 'Admins and the member always see it. Hidden ones are not shown on profiles.'})}
                value={draft.visibility === 'always' ? 'when_set' : draft.visibility}
                options={[
                    ['when_set', formatMessage({id: 'fusion.communityAttributes.shownEveryone', defaultMessage: 'Everyone'})],
                    ['admin', formatMessage({id: 'fusion.communityAttributes.shownAdmins', defaultMessage: 'Admins'})],
                    ['hidden', formatMessage({id: 'fusion.communityAttributes.shownNobody', defaultMessage: 'Hidden'})],
                ]}
                onChange={(value) => set({visibility: value === 'when_set' && field?.attrs?.visibility === 'always' ? 'always' : value})}
            />
            <Toggle
                title={formatMessage({id: 'fusion.communityAttributes.memberList', defaultMessage: 'Show in the member list'})}
                desc={formatMessage({id: 'fusion.communityAttributes.memberListDesc', defaultMessage: 'Under people\'s names, in place of their position, and to filter the list by.'})}
                on={draft.memberList}
                onChange={(memberList) => set({memberList})}
            />
            <div className={am('modal-actions')}>
                <button
                    type='button'
                    className={am('btn')}
                    onClick={onCancel}
                >
                    {formatMessage({id: 'fusion.communityAttributes.cancel', defaultMessage: 'Cancel'})}
                </button>
                <button
                    type='button'
                    className={am('btn', 'primary')}
                    disabled={!valid}
                    onClick={() => onSave(draft)}
                >
                    {field ? formatMessage({id: 'fusion.communityAttributes.save', defaultMessage: 'Save'}) : formatMessage({id: 'fusion.communityAttributes.createIt', defaultMessage: 'Add attribute'})}
                </button>
            </div>
        </div>
    );
}
