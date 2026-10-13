// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {CustomEmoji} from '@mattermost/types/emojis';

import {createCustomEmoji, deleteCustomEmoji, getCustomEmojis, searchCustomEmojis} from 'mattermost-redux/actions/emojis';
import {Client4} from 'mattermost-redux/client';
import {Permissions} from 'mattermost-redux/constants';
import {getCustomEmojis as selectCustomEmojis} from 'mattermost-redux/selectors/entities/emojis';
import {getTeammateNameDisplaySetting} from 'mattermost-redux/selectors/entities/preferences';
import {haveICurrentTeamPermission} from 'mattermost-redux/selectors/entities/roles';
import {getCurrentUserId, getUsers} from 'mattermost-redux/selectors/entities/users';
import {displayUsername} from 'mattermost-redux/utils/user_utils';

import {getEmojiMap} from 'selectors/emojis';

import Icon from 'fusion/components/icon';
import {Dialog} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// The largest image, as on the server (model.MaxEmojiFileSize).
const MAX_BYTES = 1 << 20;
const PAGE = 200;
const ACCEPT = 'image/png,image/gif,image/jpeg,image/webp,.png,.gif,.jpg,.jpeg,.webp';

// nameFromFile makes an emoji name of a file name: "Party Parrot.gif" is "party_parrot".
export function nameFromFile(file: string) {
    return file.replace(/\.[^.]+$/, '').toLowerCase().replace(/[^a-z0-9+_-]+/g, '_').replace(/^_+|_+$/g, '').slice(0, 64);
}

type Tab = 'add' | 'all';

type Props = {

    // The tab it opens on: adding one (from the pickers' "+") or the server's custom emoji.
    tab?: Tab;

    // From the modal controller.
    onExited: () => void;
};

// CustomEmojiDialog adds and manages the server's custom emoji, like the GIF and sticker uploads of the pickers,
// instead of the classic backstage pages.
export default function CustomEmojiDialog({tab: initialTab = 'add', onExited}: Props) {
    const {formatMessage} = useIntl();
    const [tab, setTab] = useState<Tab>(initialTab);
    const title = formatMessage({id: 'fusion.customEmoji.title', defaultMessage: 'Custom emoji'});

    return (
        <Dialog
            label={title}
            onClose={onExited}
            className={am('emoji-admin')}
        >
            <div className={am('pane')}>
                <h2>{title}</h2>
                <div
                    className={am('seg')}
                    role='tablist'
                >
                    <button
                        type='button'
                        role='tab'
                        aria-selected={tab === 'add'}
                        className={am({on: tab === 'add'})}
                        onClick={() => setTab('add')}
                    >
                        {formatMessage({id: 'fusion.customEmoji.addTab', defaultMessage: 'Add'})}
                    </button>
                    <button
                        type='button'
                        role='tab'
                        aria-selected={tab === 'all'}
                        className={am({on: tab === 'all'})}
                        onClick={() => setTab('all')}
                    >
                        {formatMessage({id: 'fusion.customEmoji.allTab', defaultMessage: 'All custom emoji'})}
                    </button>
                </div>
                {tab === 'add' ? <AddEmoji onClose={onExited}/> : <EmojiList onClose={onExited}/>}
            </div>
        </Dialog>
    );
}

function AddEmoji({onClose}: {onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const emojiMap = useSelector(getEmojiMap);
    const me = useSelector(getCurrentUserId);
    const [file, setFile] = useState<File | null>(null);
    const [name, setName] = useState('');
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const [added, setAdded] = useState('');
    const fileRef = useRef<HTMLInputElement>(null);
    const preview = useMemo(() => (file ? URL.createObjectURL(file) : ''), [file]);
    useEffect(() => () => {
        if (preview) {
            URL.revokeObjectURL(preview);
        }
    }, [preview]);

    // The name as the server takes it: lowercase, without the colons people type around it.
    const clean = name.trim().toLowerCase().replace(/^:(.*):$/, '$1');

    const problem = (): string => {
        if (!clean) {
            return formatMessage({id: 'fusion.customEmoji.nameRequired', defaultMessage: 'Give the emoji a name.'});
        }
        if ((/[^a-z0-9+_-]/).test(clean)) {
            return formatMessage({id: 'fusion.customEmoji.nameInvalid', defaultMessage: 'A name can only have lowercase letters, numbers, and - + _'});
        }
        if (emojiMap.hasSystemEmoji(clean)) {
            return formatMessage({id: 'fusion.customEmoji.nameSystem', defaultMessage: 'An emoji already has this name.'});
        }
        if (emojiMap.has(clean)) {
            return formatMessage({id: 'fusion.customEmoji.nameTaken', defaultMessage: 'A custom emoji already has this name.'});
        }
        if (!file) {
            return formatMessage({id: 'fusion.customEmoji.imageRequired', defaultMessage: 'Choose an image.'});
        }
        if (file.size > MAX_BYTES) {
            return formatMessage({id: 'fusion.customEmoji.tooLarge', defaultMessage: 'The image must be 1 MB at most.'});
        }
        return '';
    };

    const submit = async () => {
        const why = problem();
        if (why) {
            setError(why);
            return;
        }
        setBusy(true);
        setError('');
        const result = await dispatch(createCustomEmoji({name: clean, creator_id: me}, file));
        setBusy(false);
        if (result && 'error' in result && result.error) {
            setError((result.error as {message?: string}).message || formatMessage({id: 'fusion.customEmoji.failed', defaultMessage: 'The emoji couldn\'t be added.'}));
            return;
        }

        // Ready for the next one.
        setAdded(clean);
        setFile(null);
        setName('');
        if (fileRef.current) {
            fileRef.current.value = '';
        }
    };

    return (
        <form
            className={am('emoji-add')}
            onSubmit={(e) => {
                e.preventDefault();
                submit();
            }}
        >
            <p className={am('lead')}>{formatMessage({id: 'fusion.customEmoji.lead', defaultMessage: 'Everyone on the server can use it in messages and reactions, as :name:. PNG, GIF (animated too), JPEG or WebP, 1 MB at most; it shows at 22 pixels high.'})}</p>
            <div className={am('emoji-add-row')}>
                <div
                    className={am('emoji-add-preview')}
                    aria-hidden={true}
                >
                    {preview ? (
                        <img
                            src={preview}
                            alt=''
                        />
                    ) : (
                        <Icon name='smile'/>
                    )}
                </div>
                <div className={am('emoji-add-fields')}>
                    <div className={am('field')}>
                        <span>{formatMessage({id: 'fusion.customEmoji.image', defaultMessage: 'Image'})}</span>

                        {/* The browser's own file input can't take the dialog's look: a button opens it. */}
                        <input
                            ref={fileRef}
                            type='file'
                            accept={ACCEPT}
                            hidden={true}
                            onChange={(e) => {
                                const f = e.target.files?.[0] || null;
                                setFile(f);
                                setError('');
                                setAdded('');
                                if (f && !name.trim()) {
                                    setName(nameFromFile(f.name));
                                }
                            }}
                        />
                        <div className={am('emoji-add-file')}>
                            <button
                                type='button'
                                className={am('btn')}
                                onClick={() => fileRef.current?.click()}
                            >
                                <Icon
                                    name='image'
                                    size='sm'
                                />
                                {file ? formatMessage({id: 'fusion.customEmoji.changeImage', defaultMessage: 'Change image…'}) : formatMessage({id: 'fusion.customEmoji.chooseImage', defaultMessage: 'Choose an image…'})}
                            </button>
                            {file && <span className={am('emoji-add-filename')}>{file.name}</span>}
                        </div>
                    </div>
                    <label className={am('field')}>
                        <span>{formatMessage({id: 'fusion.customEmoji.name', defaultMessage: 'Name'})}</span>
                        <input
                            type='text'
                            value={name}
                            maxLength={66}
                            placeholder={formatMessage({id: 'fusion.customEmoji.namePlaceholder', defaultMessage: 'e.g. party_parrot'})}
                            onChange={(e) => {
                                setName(e.target.value);
                                setError('');
                                setAdded('');
                            }}
                        />
                    </label>
                </div>
            </div>
            {error && (
                <p
                    className={am('lead', 'emoji-add-error')}
                    role='alert'
                >
                    {error}
                </p>
            )}
            {added && !error && (
                <p
                    className={am('lead', 'emoji-add-done')}
                    role='status'
                >
                    {formatMessage({id: 'fusion.customEmoji.added', defaultMessage: ':{name}: is ready to use.'}, {name: added})}
                </p>
            )}
            <div className={am('modal-actions')}>
                <button
                    type='button'
                    className={am('btn')}
                    onClick={onClose}
                >
                    {added ? formatMessage({id: 'fusion.customEmoji.done', defaultMessage: 'Done'}) : formatMessage({id: 'fusion.customEmoji.cancel', defaultMessage: 'Cancel'})}
                </button>
                <button
                    className={am('btn', 'primary')}
                    disabled={busy || !file || !clean}
                >
                    {busy ? formatMessage({id: 'fusion.customEmoji.adding', defaultMessage: 'Adding…'}) : formatMessage({id: 'fusion.customEmoji.add', defaultMessage: 'Add emoji'})}
                </button>
            </div>
        </form>
    );
}

function EmojiList({onClose}: {onClose: () => void}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const all = useSelector(selectCustomEmojis);
    const users = useSelector(getUsers);
    const nameDisplay = useSelector(getTeammateNameDisplaySetting);
    const me = useSelector(getCurrentUserId);
    const canDeleteOwn = useSelector((state: GlobalState) => haveICurrentTeamPermission(state, Permissions.DELETE_EMOJIS));
    const canDeleteOthers = useSelector((state: GlobalState) => haveICurrentTeamPermission(state, Permissions.DELETE_OTHERS_EMOJIS));
    const [query, setQuery] = useState('');
    const [pages, setPages] = useState(0);
    const [more, setMore] = useState(true);
    const [confirming, setConfirming] = useState('');
    const [error, setError] = useState('');

    // The server's emoji come a page at a time, sorted by name; a search asks the server too.
    useEffect(() => {
        (async () => {
            const result = await dispatch(getCustomEmojis(pages, PAGE, 'name', true));
            if (result && 'data' in result && result.data) {
                setMore(result.data.length === PAGE);
            }
        })();
    }, [pages, dispatch]);
    useEffect(() => {
        const term = query.trim().replace(/^:|:$/g, '');
        if (!term) {
            return undefined;
        }
        const t = setTimeout(() => dispatch(searchCustomEmojis(term, {}, true)), 250);
        return () => clearTimeout(t);
    }, [query, dispatch]);

    const term = query.trim().toLowerCase().replace(/^:|:$/g, '');
    const list = Object.values(all).
        filter((e: CustomEmoji) => !term || e.name.includes(term)).
        sort((a: CustomEmoji, b: CustomEmoji) => a.name.localeCompare(b.name));

    const remove = async (emoji: CustomEmoji) => {
        setError('');
        const result = await dispatch(deleteCustomEmoji(emoji.id));
        setConfirming('');
        if (result && 'error' in result && result.error) {
            setError((result.error as {message?: string}).message || formatMessage({id: 'fusion.customEmoji.deleteFailed', defaultMessage: 'The emoji couldn\'t be deleted.'}));
        }
    };

    return (
        <div className={am('emoji-list')}>
            <label className={am('emoji-list-search')}>
                <Icon
                    name='search'
                    size='sm'
                />
                <input
                    type='text'
                    value={query}
                    placeholder={formatMessage({id: 'fusion.customEmoji.search', defaultMessage: 'Search custom emoji'})}
                    aria-label={formatMessage({id: 'fusion.customEmoji.search', defaultMessage: 'Search custom emoji'})}
                    autoComplete='off'
                    onChange={(e) => setQuery(e.target.value)}
                />
            </label>
            {error && (
                <p
                    className={am('lead', 'emoji-add-error')}
                    role='alert'
                >
                    {error}
                </p>
            )}
            <ul className={am('emoji-rows')}>
                {list.map((emoji: CustomEmoji) => {
                    const creator = users[emoji.creator_id];
                    const canDelete = emoji.creator_id === me ? canDeleteOwn : canDeleteOthers;
                    return (
                        <li key={emoji.id}>
                            <img
                                src={Client4.getCustomEmojiImageUrl(emoji.id)}
                                alt=''
                            />
                            <span className={am('emoji-row-name')}>{`:${emoji.name}:`}</span>
                            <span className={am('emoji-row-by')}>{creator ? displayUsername(creator, nameDisplay) : ''}</span>
                            {canDelete && confirming !== emoji.id && (
                                <button
                                    type='button'
                                    className={am('icon-btn')}
                                    title={formatMessage({id: 'fusion.customEmoji.delete', defaultMessage: 'Delete :{name}:'}, {name: emoji.name})}
                                    aria-label={formatMessage({id: 'fusion.customEmoji.delete', defaultMessage: 'Delete :{name}:'}, {name: emoji.name})}
                                    onClick={() => setConfirming(emoji.id)}
                                >
                                    <Icon
                                        name='trash'
                                        size='sm'
                                    />
                                </button>
                            )}
                            {confirming === emoji.id && (
                                <span className={am('emoji-row-confirm')}>
                                    <button
                                        type='button'
                                        className={am('btn', 'small')}
                                        onClick={() => setConfirming('')}
                                    >
                                        {formatMessage({id: 'fusion.customEmoji.keep', defaultMessage: 'Keep'})}
                                    </button>
                                    <button
                                        type='button'
                                        className={am('btn', 'small', 'danger')}
                                        onClick={() => remove(emoji)}
                                    >
                                        {formatMessage({id: 'fusion.customEmoji.confirmDelete', defaultMessage: 'Delete'})}
                                    </button>
                                </span>
                            )}
                        </li>
                    );
                })}
            </ul>
            {!list.length && (
                <p className={am('lead')}>
                    {term ? formatMessage({id: 'fusion.customEmoji.noMatch', defaultMessage: 'No custom emoji matches.'}) : formatMessage({id: 'fusion.customEmoji.none', defaultMessage: 'No custom emoji yet.'})}
                </p>
            )}
            <div className={am('modal-actions')}>
                {more && !term && (
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={() => setPages(pages + 1)}
                    >
                        {formatMessage({id: 'fusion.customEmoji.more', defaultMessage: 'Show more'})}
                    </button>
                )}
                <button
                    type='button'
                    className={am('btn')}
                    onClick={onClose}
                >
                    {formatMessage({id: 'fusion.customEmoji.close', defaultMessage: 'Close'})}
                </button>
            </div>
        </div>
    );
}
