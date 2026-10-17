// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useState} from 'react';
import {useIntl} from 'react-intl';
import type {IntlShape} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {CustomEmoji} from '@mattermost/types/emojis';

import {createCustomEmoji, deleteCustomEmoji, getCustomEmojiByName, getCustomEmojis, searchCustomEmojis} from 'mattermost-redux/actions/emojis';
import {Client4} from 'mattermost-redux/client';
import {getCustomEmojis as selectCustomEmojis} from 'mattermost-redux/selectors/entities/emojis';
import {getTeammateNameDisplaySetting} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUserId, getUsers} from 'mattermost-redux/selectors/entities/users';
import {displayUsername} from 'mattermost-redux/utils/user_utils';

import {getEmojiMap} from 'selectors/emojis';

import {canCreateCustomEmoji, canDeleteOthersCustomEmoji, canDeleteOwnCustomEmoji, canManageCustomEmoji} from 'fusion/selectors/custom_emoji';
import {am} from 'fusion/utils/class_names';

import {PickerDropZone, PickerField, PickerManageList, PickerNotice, PickerSearch, PickerViewButtons, PickerViewFrame} from './picker_views';
import type {PickerListItem} from './picker_views';

// The custom emoji views of the emoji picker ("+" adds one, "Custom emoji" manages them), also the /:team/emoji page.
// The limits are the server's (model/emoji.go, app/emoji.go), so that what the view accepts, the server takes.

// The longest name (model.EmojiNameMaxLength).
export const NAME_MAX = 64;

// The largest image (app.MaxEmojiFileSize: 512 KiB), its largest size in pixels (MaxEmojiOriginalWidth/Height; it's
// made 128 pixels at most) and the most frames of an animated GIF (MaxEmojiGIFFrames).
export const MAX_BYTES = 1 << 19;
export const MAX_PIXELS = 1028;
export const MAX_FRAMES = 70;

// The images the server decodes: PNG, GIF (animated too), JPEG and WebP.
const TYPES = ['image/png', 'image/gif', 'image/jpeg', 'image/webp'];
const EXTENSIONS = /\.(png|gif|jpe?g|webp)$/i;
export const ACCEPT = 'image/png,image/gif,image/jpeg,image/webp,.png,.gif,.jpg,.jpeg,.webp';

// The sizes an emoji shows at: in reactions, in messages (and the picker), and in the picker's footer.
const PREVIEW_SIZES = [16, 22, 32];

// The page of the server's custom emoji the manage view loads at a time.
const PAGE = 100;

// nameFromFile makes an emoji name of a file name: "Party Parrot.gif" is "party_parrot".
export function nameFromFile(file: string) {
    return file.replace(/\.[^.]+$/, '').toLowerCase().replace(/[^a-z0-9+_-]+/g, '_').replace(/^_+|_+$/g, '').slice(0, NAME_MAX);
}

// cleanName is the name as the server keeps it: lowercase (it lowers it too), without the colons people type around
// emoji names.
export function cleanName(name: string) {
    return name.trim().toLowerCase().replace(/^:+|:+$/g, '');
}

export type NameProblem =
    {kind: 'empty'} |
    {kind: 'chars'; chars: string} |
    {kind: 'long'} |
    {kind: 'system'} |
    {kind: 'taken'};

// nameProblem checks a cleaned name as the server will: its characters (letters, digits, - + _), its length, and
// that no standard or custom emoji has it. The server's own check of taken names comes on top (useNameTaken).
export function nameProblem(name: string, emojiMap: {hasSystemEmoji: (n: string) => boolean; has: (n: string) => boolean}): NameProblem | null {
    if (!name) {
        return {kind: 'empty'};
    }
    const bad = Array.from(new Set(name.replace(/[a-z0-9+_-]/g, ''))).join('');
    if (bad) {
        return {kind: 'chars', chars: bad};
    }
    if (name.length > NAME_MAX) {
        return {kind: 'long'};
    }
    if (emojiMap.hasSystemEmoji(name)) {
        return {kind: 'system'};
    }
    if (emojiMap.has(name)) {
        return {kind: 'taken'};
    }
    return null;
}

function nameMessage(problem: NameProblem, name: string, intl: IntlShape): string {
    switch (problem.kind) {
    case 'chars':
        return intl.formatMessage({id: 'fusion.customEmoji.name.chars', defaultMessage: 'A name can’t have {chars}. Use lowercase letters, numbers, and - + _'}, {chars: problem.chars.split('').map((c) => (c === ' ' ? intl.formatMessage({id: 'fusion.customEmoji.name.space', defaultMessage: 'spaces'}) : `“${c}”`)).join(' ')});
    case 'long':
        return intl.formatMessage({id: 'fusion.customEmoji.name.long', defaultMessage: 'A name can have up to {max} characters.'}, {max: NAME_MAX});
    case 'system':
        return intl.formatMessage({id: 'fusion.customEmoji.name.system', defaultMessage: ':{name}: is a standard emoji. Choose another name.'}, {name});
    case 'taken':
        return intl.formatMessage({id: 'fusion.customEmoji.name.taken', defaultMessage: 'There’s already a custom emoji called :{name}:.'}, {name});
    default:
        return '';
    }
}

export type ImageProblem = {kind: 'type'} | {kind: 'size'} | {kind: 'pixels'};

// imageProblem checks an image's type and file size; its size in pixels is checked once it's loaded.
export function imageProblem(file: File): ImageProblem | null {
    if (!TYPES.includes(file.type) && !EXTENSIONS.test(file.name)) {
        return {kind: 'type'};
    }
    if (file.size > MAX_BYTES) {
        return {kind: 'size'};
    }
    return null;
}

function imageMessage(problem: ImageProblem, intl: IntlShape): string {
    switch (problem.kind) {
    case 'type':
        return intl.formatMessage({id: 'fusion.customEmoji.image.type', defaultMessage: 'Choose a PNG, GIF, JPEG or WebP image.'});
    case 'size':
        return intl.formatMessage({id: 'fusion.customEmoji.image.size', defaultMessage: 'This image is larger than {max} KB. Choose a smaller one.'}, {max: MAX_BYTES >> 10});
    default:
        return intl.formatMessage({id: 'fusion.customEmoji.image.pixels', defaultMessage: 'This image is larger than {max} × {max} pixels. Choose a smaller one.'}, {max: MAX_PIXELS});
    }
}

// useNameTaken asks the server, a moment after typing stops, whether a custom emoji has the name already: the picker
// knows only the custom emoji it loaded. undefined while it doesn't know.
function useNameTaken(name: string, check: boolean): boolean | undefined {
    const dispatch = useDispatch();
    const [known, setKnown] = useState<Record<string, boolean>>({});
    useEffect(() => {
        if (!check || name in known) {
            return undefined;
        }
        let gone = false;
        const t = setTimeout(async () => {
            const result = await dispatch(getCustomEmojiByName(name));
            if (gone) {
                return;
            }
            const status = (result as {error?: {status_code?: number}})?.error?.status_code;

            // Not found: free. Another error (offline...): the server says so on Save.
            if (result && 'data' in result && result.data) {
                setKnown((k) => ({...k, [name]: true}));
            } else if (status === 404) {
                setKnown((k) => ({...k, [name]: false}));
            }
        }, 350);
        return () => {
            gone = true;
            clearTimeout(t);
        };
    }, [name, check, known, dispatch]);
    return check ? known[name] : undefined;
}

// EmojiPreview shows the chosen image at the sizes emoji appear at.
function EmojiPreview({url, name}: {url: string; name: string}) {
    const {formatMessage} = useIntl();
    const labels = [
        formatMessage({id: 'fusion.customEmoji.preview.reaction', defaultMessage: 'Reaction'}),
        formatMessage({id: 'fusion.customEmoji.preview.message', defaultMessage: 'Message'}),
        formatMessage({id: 'fusion.customEmoji.preview.large', defaultMessage: 'Large'}),
    ];
    return (
        <div
            className={am('ce-preview')}
            role='img'
            aria-label={formatMessage({id: 'fusion.customEmoji.preview.label', defaultMessage: 'Preview of :{name}: at {sizes} pixels'}, {name: name || '…', sizes: PREVIEW_SIZES.join(', ')})}
        >
            {PREVIEW_SIZES.map((size, i) => (
                <figure key={size}>
                    <img
                        src={url}
                        alt=''
                        width={size}
                        height={size}
                    />
                    <figcaption>{labels[i]}</figcaption>
                </figure>
            ))}
        </div>
    );
}

type AddProps = {
    onBack: () => void;

    // Once it's added: the picker goes back to its grid with it ready to use.
    onAdded: (emoji: CustomEmoji) => void;
    variant?: 'picker' | 'page';
};

// AddCustomEmojiView adds a custom emoji: an image (chosen, dropped or pasted) and a name, checked as they're given.
export function AddCustomEmojiView({onBack, onAdded, variant = 'picker'}: AddProps) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const dispatch = useDispatch();
    const emojiMap = useSelector(getEmojiMap);
    const me = useSelector(getCurrentUserId);
    const [file, setFile] = useState<File | null>(null);
    const [pixels, setPixels] = useState<{w: number; h: number} | null>(null);
    const [name, setName] = useState('');
    const [touched, setTouched] = useState(false);
    const [busy, setBusy] = useState(false);
    const [error, setError] = useState('');
    const [tried, setTried] = useState(false);

    const url = useMemo(() => (file ? URL.createObjectURL(file) : ''), [file]);
    useEffect(() => () => {
        if (url) {
            URL.revokeObjectURL(url);
        }
    }, [url]);

    const clean = cleanName(name);
    const problem = nameProblem(clean, emojiMap);
    const takenOnServer = useNameTaken(clean, !problem);
    const nameIssue: NameProblem | null = problem || (takenOnServer ? {kind: 'taken'} : null);
    const showName = touched || tried;
    const nameError = nameIssue && nameIssue.kind !== 'empty' ? nameMessage(nameIssue, clean, intl) : '';
    const nameEmpty = tried && nameIssue?.kind === 'empty' ? formatMessage({id: 'fusion.customEmoji.name.required', defaultMessage: 'Give the emoji a name.'}) : '';

    let imageIssue: ImageProblem | null = file ? imageProblem(file) : null;
    if (!imageIssue && pixels && (pixels.w > MAX_PIXELS || pixels.h > MAX_PIXELS)) {
        imageIssue = {kind: 'pixels'};
    }
    let imageError = imageIssue ? imageMessage(imageIssue, intl) : '';
    if (!file && tried) {
        imageError = formatMessage({id: 'fusion.customEmoji.image.required', defaultMessage: 'Choose an image.'});
    }

    const ready = Boolean(file) && !imageIssue && !nameIssue && takenOnServer !== true;

    const choose = (f: File) => {
        setFile(f);
        setPixels(null);
        setError('');
        if (!name.trim()) {
            setName(nameFromFile(f.name));
        }
    };

    // Its size in pixels, once it's loaded (browsers only: the server checks it anyway).
    useEffect(() => {
        if (!url || typeof Image === 'undefined') {
            return undefined;
        }
        const img = new Image();
        img.onload = () => setPixels({w: img.naturalWidth, h: img.naturalHeight});
        img.src = url;
        return () => {
            img.onload = null;
        };
    }, [url]);

    const save = async () => {
        setTried(true);
        if (!ready || busy) {
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
        const created = (result as {data?: CustomEmoji}).data;
        if (created) {
            onAdded(created);
        }
    };

    return (
        <PickerViewFrame
            title={formatMessage({id: 'fusion.customEmoji.addTitle', defaultMessage: 'Add a custom emoji'})}
            onBack={onBack}
            variant={variant}
            onSubmit={save}
            actions={(
                <>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={onBack}
                    >
                        {formatMessage({id: 'fusion.customEmoji.cancel', defaultMessage: 'Cancel'})}
                    </button>
                    <button
                        type='submit'
                        className={am('btn', 'primary')}
                        disabled={busy}
                        aria-disabled={!ready}
                    >
                        {busy ? formatMessage({id: 'fusion.customEmoji.adding', defaultMessage: 'Adding…'}) : formatMessage({id: 'fusion.customEmoji.save', defaultMessage: 'Save'})}
                    </button>
                </>
            )}
        >
            <PickerDropZone
                accept={ACCEPT}
                file={file}
                onFile={choose}
                label={formatMessage({id: 'fusion.customEmoji.image.choose', defaultMessage: 'Drop, paste or choose an image'})}
                changeLabel={formatMessage({id: 'fusion.customEmoji.image.change', defaultMessage: 'Change image…'})}
                hint={formatMessage({id: 'fusion.customEmoji.image.hint', defaultMessage: 'PNG, GIF (animated: up to {frames} frames), JPEG or WebP; up to {kb} KB and {px} × {px} pixels. Shown at 128 pixels at most.'}, {frames: MAX_FRAMES, kb: MAX_BYTES >> 10, px: MAX_PIXELS})}
                error={imageError}
                preview={url ? (
                    <EmojiPreview
                        url={url}
                        name={clean}
                    />
                ) : undefined}
            />
            <PickerField
                label={formatMessage({id: 'fusion.customEmoji.name', defaultMessage: 'Name'})}
                aside={`${clean.length}/${NAME_MAX}`}
                hint={formatMessage({id: 'fusion.customEmoji.name.hint', defaultMessage: 'Lowercase letters, numbers, and - + _. People type it as :name:.'})}
                error={showName || clean ? (nameError || nameEmpty) : ''}
                ok={!nameIssue && takenOnServer === false ? formatMessage({id: 'fusion.customEmoji.name.free', defaultMessage: ':{name}: is free.'}, {name: clean}) : ''}
            >
                {({id, describedBy, invalid}) => (
                    <span className={am('ce-name')}>
                        <span aria-hidden='true'>{':'}</span>
                        <input
                            id={id}
                            type='text'
                            value={name}
                            autoComplete='off'
                            spellCheck={false}
                            maxLength={NAME_MAX + 8}
                            aria-describedby={describedBy}
                            aria-invalid={invalid}
                            placeholder={formatMessage({id: 'fusion.customEmoji.nameExample', defaultMessage: 'party_parrot'})}
                            onChange={(e) => {
                                setName(e.target.value);
                                setError('');
                            }}
                            onBlur={() => setTouched(true)}
                        />
                        <span aria-hidden='true'>{':'}</span>
                    </span>
                )}
            </PickerField>
            <PickerNotice>{error}</PickerNotice>
        </PickerViewFrame>
    );
}

type ManageProps = {
    onBack: () => void;

    // The "Add" button of the page; the picker has its "+" for it.
    onAdd?: () => void;

    // Picking one, e.g. to react with it.
    onPick?: (emoji: CustomEmoji) => void;
    variant?: 'picker' | 'page';
    noBack?: boolean;

    // The search it opens with (the page, after adding one).
    initialQuery?: string;
};

// ManageCustomEmojiView lists the server's custom emoji (they're everyone's, in every community): searched, with who
// added each and when, and a delete button where the person may delete it.
export function ManageCustomEmojiView({onBack, onAdd, onPick, variant = 'picker', noBack, initialQuery = ''}: ManageProps) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const dispatch = useDispatch();
    const all = useSelector(selectCustomEmojis);
    const users = useSelector(getUsers);
    const nameDisplay = useSelector(getTeammateNameDisplaySetting);
    const me = useSelector(getCurrentUserId);
    const canDeleteOwn = useSelector(canDeleteOwnCustomEmoji);
    const canDeleteOthers = useSelector(canDeleteOthersCustomEmoji);
    const canCreate = useSelector(canCreateCustomEmoji);
    const [query, setQuery] = useState(initialQuery);
    const [page, setPage] = useState(0);
    const [more, setMore] = useState(true);
    const [loading, setLoading] = useState(false);
    const [searching, setSearching] = useState(false);

    // The server's emoji come a page at a time, sorted by name, as the list scrolls.
    useEffect(() => {
        let gone = false;
        setLoading(true);
        (async () => {
            const result = await dispatch(getCustomEmojis(page, PAGE, 'name', true));
            if (gone) {
                return;
            }
            setLoading(false);
            if (result && 'data' in result && result.data) {
                setMore(result.data.length === PAGE);
            } else {
                setMore(false);
            }
        })();
        return () => {
            gone = true;
        };
    }, [page, dispatch]);

    // A search asks the server too, for the emoji not loaded yet.
    const term = cleanName(query);
    useEffect(() => {
        if (!term) {
            setSearching(false);
            return undefined;
        }
        setSearching(true);
        const t = setTimeout(async () => {
            await dispatch(searchCustomEmojis(term, {}, true));
            setSearching(false);
        }, 250);
        return () => clearTimeout(t);
    }, [term, dispatch]);

    const emojis = useMemo(() => Object.values(all).
        filter((e: CustomEmoji) => !term || e.name.includes(term)).
        sort((a: CustomEmoji, b: CustomEmoji) => a.name.localeCompare(b.name)), [all, term]);
    const byId = useMemo(() => new Map(emojis.map((e) => [e.id, e])), [emojis]);

    const items: PickerListItem[] = emojis.map((emoji) => {
        const creator = users[emoji.creator_id];
        return {
            id: emoji.id,
            name: `:${emoji.name}:`,
            image: (
                <img
                    src={Client4.getCustomEmojiImageUrl(emoji.id)}
                    alt=''
                    loading='lazy'
                />
            ),
            by: creator ? displayUsername(creator, nameDisplay) : undefined,
            at: emoji.create_at || undefined,
            canDelete: emoji.creator_id === me ? canDeleteOwn : canDeleteOthers,
        };
    });

    const remove = async (item: PickerListItem) => {
        const result = await dispatch(deleteCustomEmoji(item.id));
        if (result && 'error' in result && result.error) {
            return (result.error as {message?: string}).message || formatMessage({id: 'fusion.customEmoji.deleteFailed', defaultMessage: 'The emoji couldn\'t be deleted.'});
        }
        return undefined;
    };
    const nameOf = (item: PickerListItem) => byId.get(item.id)?.name || item.name.replace(/:/g, '');

    let empty = formatMessage({id: 'fusion.customEmoji.none', defaultMessage: 'No custom emoji yet.'});
    if (term) {
        empty = formatMessage({id: 'fusion.customEmoji.noMatchTerm', defaultMessage: 'No custom emoji matches “{term}”.'}, {term});
    }

    return (
        <PickerViewFrame
            title={formatMessage({id: 'fusion.customEmoji.title', defaultMessage: 'Custom emoji'})}
            onBack={onBack}
            variant={variant}
            noBack={noBack}
            tools={onAdd && canCreate ? (
                <button
                    type='button'
                    className={am('btn', 'primary', 'small')}
                    onClick={onAdd}
                >
                    {formatMessage({id: 'fusion.customEmoji.addButton', defaultMessage: 'Add custom emoji'})}
                </button>
            ) : undefined}
        >
            <PickerSearch
                value={query}
                onChange={setQuery}
                placeholder={formatMessage({id: 'fusion.customEmoji.search', defaultMessage: 'Search custom emoji'})}
            />
            <PickerManageList
                items={items}
                onDelete={remove}
                onOpen={onPick ? (item) => {
                    const emoji = byId.get(item.id);
                    if (emoji) {
                        onPick(emoji);
                    }
                } : undefined}
                deleteLabel={(item) => formatMessage({id: 'fusion.customEmoji.delete', defaultMessage: 'Delete :{name}:'}, {name: nameOf(item)})}
                confirmText={(item) => formatMessage({id: 'fusion.customEmoji.confirm', defaultMessage: 'Delete :{name}:? Messages and reactions using it will show its name instead.'}, {name: nameOf(item)})}
                deletedText={(item) => formatMessage({id: 'fusion.customEmoji.deleted', defaultMessage: ':{name}: was deleted.'}, {name: nameOf(item)})}
                empty={empty}
                loading={(loading && !emojis.length) || (searching && !emojis.length)}
                hasMore={more && !term}
                onLoadMore={() => setPage((p) => p + 1)}
            />
        </PickerViewFrame>
    );
}

// The views of the emoji tab, and what opens them.
export type CustomEmojiView = 'add' | 'manage';

type ButtonsProps = {
    onOpen: (view: CustomEmojiView) => void;
};

// CustomEmojiButtons are the emoji tab's "+" (add) and "Custom emoji" (manage) buttons, for those allowed to use
// them, when custom emoji are on.
export function CustomEmojiButtons({onOpen}: ButtonsProps) {
    const {formatMessage} = useIntl();
    const canCreate = useSelector(canCreateCustomEmoji);
    const canManage = useSelector(canManageCustomEmoji);
    return (
        <PickerViewButtons
            onAdd={canCreate ? () => onOpen('add') : undefined}
            onManage={canManage ? () => onOpen('manage') : undefined}
            addLabel={formatMessage({id: 'fusion.picker.addCustomEmoji', defaultMessage: 'Add custom emoji'})}
            manageLabel={formatMessage({id: 'fusion.picker.manageCustomEmoji', defaultMessage: 'Custom emoji'})}
        />
    );
}
