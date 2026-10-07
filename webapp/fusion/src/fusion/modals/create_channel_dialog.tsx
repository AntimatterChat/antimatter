// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {Channel, ChannelType} from '@mattermost/types/channels';
import type {NewChannelFormState} from '@mattermost/types/plugins';

import {addChannelToCategory} from 'mattermost-redux/actions/channel_categories';
import {createChannel} from 'mattermost-redux/actions/channels';
import {Permissions} from 'mattermost-redux/constants';
import {haveITeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';

import {switchToChannel} from 'actions/views/channel';
import {getCategoriesForCurrentTeam} from 'selectors/views/channel_sidebar';

import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import Constants from 'utils/constants';
import {cleanUpUrlable} from 'utils/url';
import {generateId} from 'utils/utils';

import type {GlobalState} from 'types/store';

type Props = {

    // The sidebar category to put the channel in (its "+" button), else the default one.
    categoryId?: string;

    // The classic dialog, for what this one leaves out (channel URL, purpose, classification…).
    onMoreOptions: () => void;
    onClose: () => void;
};

const TEXT = 'text';

// CreateChannelDialog creates a channel: the mockup's "Create channel" dialog (type, name, category, private).
// Channel types registered by plugins (registerChannelTypeOption) appear beside text channels and create through
// the plugin, as in the classic dialog.
export default function CreateChannelDialog({categoryId, onMoreOptions, onClose}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const toast = useToast();
    const team = useSelector(getCurrentTeam);
    const categories = useSelector(getCategoriesForCurrentTeam).filter((c) => c.type !== 'direct_messages' && c.type !== 'favorites');
    const canPublic = useSelector((state: GlobalState) => Boolean(team) && haveITeamPermission(state, team!.id, Permissions.CREATE_PUBLIC_CHANNEL));
    const canPrivate = useSelector((state: GlobalState) => Boolean(team) && haveITeamPermission(state, team!.id, Permissions.CREATE_PRIVATE_CHANNEL));
    const options = useSelector((state: GlobalState) => (state.plugins.components.ChannelTypeOption || []).filter((o) => {
        try {
            return o.isAvailable(state);
        } catch {
            return false;
        }
    }), shallowEqual);
    const [kind, setKind] = useState(TEXT);
    const [name, setName] = useState('');
    const [category, setCategory] = useState(categoryId || categories.find((c) => c.type === 'channels')?.id || '');
    const [isPrivate, setPrivate] = useState(!canPublic && canPrivate);
    const [pluginCanCreate, setPluginCanCreate] = useState(true);
    const [saving, setSaving] = useState(false);
    const [error, setError] = useState('');
    const nameRef = useRef<HTMLInputElement>(null);

    useEffect(() => {
        nameRef.current?.focus();
    }, []);

    const plugin = options.find((o) => o.id === kind);
    const type: ChannelType = isPrivate ? Constants.PRIVATE_CHANNEL as ChannelType : Constants.OPEN_CHANNEL as ChannelType;
    const url = cleanUpUrlable(name) || generateId().slice(0, 26);
    const formState: NewChannelFormState = {teamId: team?.id || '', displayName: name.trim(), url, purpose: '', type: plugin ? plugin.id : type, privacy: type} as NewChannelFormState;

    const done = (channel: Channel) => {
        if (category && category !== categories.find((c) => c.type === 'channels')?.id) {
            dispatch(addChannelToCategory(category, channel.id));
        }
        dispatch(switchToChannel(channel));
        toast(formatMessage({id: 'fusion.toast.created', defaultMessage: 'Created {name}'}, {name: plugin ? channel.display_name : '#' + channel.display_name}));
        onClose();
    };

    const create = async () => {
        if (!team || saving) {
            return;
        }
        if (!name.trim()) {
            setError(formatMessage({id: 'fusion.createChannel.noName', defaultMessage: 'Give the channel a name'}));
            nameRef.current?.focus();
            return;
        }
        setSaving(true);
        setError('');
        if (plugin) {
            try {
                const result = await plugin.onCreate(formState);
                if (result?.status === 'created' && result.channel) {
                    done(result.channel);
                } else if (result?.status === 'deferred') {
                    onClose();
                } else {
                    setError((result?.status === 'error' && result.message) || formatMessage({id: 'fusion.createChannel.failed', defaultMessage: 'Something went wrong. Please try again.'}));
                }
            } catch {
                setError(formatMessage({id: 'fusion.createChannel.failed', defaultMessage: 'Something went wrong. Please try again.'}));
            }
            setSaving(false);
            return;
        }
        const channel = {team_id: team.id, name: url, display_name: name.trim(), purpose: '', header: '', type} as Channel;
        const result = await dispatch(createChannel(channel, ''));
        setSaving(false);
        if (result && 'error' in result && result.error) {
            setError((result.error as {message?: string}).message || formatMessage({id: 'fusion.createChannel.failed', defaultMessage: 'Something went wrong. Please try again.'}));
            return;
        }
        if (result && 'data' in result && result.data) {
            done(result.data);
        }
    };

    const title = formatMessage({id: 'fusion.createChannel.title', defaultMessage: 'Create channel'});
    const PluginExtra = plugin?.extraContent;
    return (
        <Dialog
            label={title}
            onClose={onClose}
            onSubmit={create}
        >
            <div className={am('pane')}>
                <h2>{title}</h2>
                <p className={am('lead')}>{formatMessage({id: 'fusion.createChannel.lead', defaultMessage: 'In {team}.'}, {team: team?.display_name})}</p>
                {options.length > 0 && (
                    <div className={am('field')}>
                        <span>{formatMessage({id: 'fusion.createChannel.type', defaultMessage: 'Channel type'})}</span>
                        <div
                            className={am('type-grid')}
                            role='radiogroup'
                            aria-label={formatMessage({id: 'fusion.createChannel.type', defaultMessage: 'Channel type'})}
                        >
                            <button
                                type='button'
                                role='radio'
                                aria-checked={kind === TEXT}
                                className={am('type-card', {on: kind === TEXT})}
                                onClick={() => setKind(TEXT)}
                            >
                                <Icon name='hash'/>
                                <span>
                                    <b>{formatMessage({id: 'fusion.createChannel.text', defaultMessage: 'Text'})}</b>
                                    <span>{formatMessage({id: 'fusion.createChannel.textDesc', defaultMessage: 'Messages, files and threads'})}</span>
                                </span>
                            </button>
                            {options.map((o) => (
                                <button
                                    key={o.id}
                                    type='button'
                                    role='radio'
                                    aria-checked={kind === o.id}
                                    className={am('type-card', {on: kind === o.id})}
                                    onClick={() => {
                                        setKind(o.id);
                                        setPluginCanCreate(true);
                                    }}
                                >
                                    <span className={am('type-ic')}>{o.icon}</span>
                                    <span>
                                        <b>{o.label}</b>
                                        <span>{o.description}</span>
                                    </span>
                                </button>
                            ))}
                        </div>
                    </div>
                )}
                <label className={am('field')}>
                    <span>{formatMessage({id: 'fusion.createChannel.name', defaultMessage: 'Name'})}</span>
                    <input
                        ref={nameRef}
                        type='text'
                        value={name}
                        maxLength={64}
                        placeholder={formatMessage({id: 'fusion.createChannel.namePlaceholder', defaultMessage: 'e.g. beam-schedule'})}
                        onChange={(e) => setName(e.target.value)}
                    />
                </label>
                {!plugin && categories.length > 1 && (
                    <label className={am('field')}>
                        <span>{formatMessage({id: 'fusion.createChannel.category', defaultMessage: 'Category'})}</span>
                        <select
                            value={category}
                            onChange={(e) => setCategory(e.target.value)}
                        >
                            {categories.map((c) => (
                                <option
                                    key={c.id}
                                    value={c.id}
                                >
                                    {c.display_name}
                                </option>
                            ))}
                        </select>
                    </label>
                )}
                {(canPublic || canPrivate) && (
                    <div className={am('toggle-row')}>
                        <div className={am('txt')}>
                            <b>{formatMessage({id: 'fusion.createChannel.private', defaultMessage: 'Private channel'})}</b>
                            <span>{formatMessage({id: 'fusion.createChannel.privateDesc', defaultMessage: 'Only invited members can see and join it.'})}</span>
                        </div>
                        <button
                            type='button'
                            className={am('switch')}
                            role='switch'
                            aria-checked={isPrivate}
                            aria-label={formatMessage({id: 'fusion.createChannel.private', defaultMessage: 'Private channel'})}
                            disabled={!(canPublic && canPrivate)}
                            onClick={() => setPrivate(!isPrivate)}
                        />
                    </div>
                )}
                {PluginExtra && (
                    <PluginExtra
                        formState={formState}
                        setCanCreate={setPluginCanCreate}
                    />
                )}
                {error && (
                    <p
                        className={am('lead')}
                        role='alert'
                        style={{color: 'var(--am-danger)', marginTop: 10}}
                    >
                        {error}
                    </p>
                )}
                <div className={am('modal-actions')}>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onMoreOptions}
                    >
                        {formatMessage({id: 'fusion.createChannel.more', defaultMessage: 'More options…'})}
                    </button>
                    <span className={am('grow')}/>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onClose}
                    >
                        {formatMessage({id: 'fusion.createChannel.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button
                        className={am('btn', 'primary')}
                        disabled={saving || (Boolean(plugin) && !pluginCanCreate) || (!plugin && !canPublic && !canPrivate)}
                    >
                        {(plugin?.createButtonText) || title}
                    </button>
                </div>
            </div>
        </Dialog>
    );
}
