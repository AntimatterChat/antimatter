// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {createContext, useCallback, useContext, useEffect, useId, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';

import Icon from 'fusion/components/icon';
import {isTouch} from 'fusion/hooks/media';
import {am} from 'fusion/utils/class_names';

// The views a picker tab shows in place of its grid, inside the picker: adding something (a custom emoji, a GIF, a
// sticker, a meme template) and managing what was added. The picker stays open, keeps its size and place, and a back
// arrow (or Escape) returns to the grid. The emoji tab uses these components, and the picker hands them to the
// plugins' tabs (EmojiPickerTabProps.pickerKit) so that every tab looks the same.

// ---------- Escape: back to the grid before closing the picker ----------

type EscapeStack = {

    // Adds what Escape does while something is shown (a view's back, a row's confirmation); returns its removal.
    push: (onEscape: () => void) => () => void;
};

const EscapeContext = createContext<EscapeStack | null>(null);

// useEscapeStack makes the stack a picker (or a page) provides: escape() runs the last handler pushed and tells
// whether there was one, so that Escape closes the picker only from its grid.
export function useEscapeStack() {
    const stack = useRef<Array<() => void>>([]);
    const value = useMemo<EscapeStack>(() => ({
        push: (onEscape) => {
            stack.current.push(onEscape);
            return () => {
                stack.current = stack.current.filter((f) => f !== onEscape);
            };
        },
    }), []);
    const escape = useCallback(() => {
        const top = stack.current[stack.current.length - 1];
        if (!top) {
            return false;
        }
        top();
        return true;
    }, []);
    return {value, escape};
}

export const PickerEscapeProvider = EscapeContext.Provider;

// useOnEscape runs onEscape on Escape while active, before what was there (the picker's close).
export function useOnEscape(active: boolean, onEscape: () => void) {
    const stack = useContext(EscapeContext);
    const ref = useRef(onEscape);
    ref.current = onEscape;
    useEffect(() => {
        if (!active || !stack) {
            return undefined;
        }
        return stack.push(() => ref.current());
    }, [active, stack]);
}

// ---------- Which view shows ----------

// usePickerView keeps which view a tab shows ('grid' or one of its own), and gives the focus back to the button that
// opened a view when it closes: back(false) leaves the focus to the tab (e.g. on what was just added).
export function usePickerView<V extends string>(): [V | 'grid', (view: V) => void, (restoreFocus?: unknown) => void] {
    const [view, setView] = useState<V | 'grid'>('grid');
    const opener = useRef<{el: HTMLElement; label: string; root: Element | null} | null>(null);
    const open = useCallback((next: V) => {
        const active = document.activeElement;
        opener.current = active instanceof HTMLElement ? {el: active, label: active.getAttribute('aria-label') || '', root: active.closest('.am-pop, .am-ce-page')} : null;
        setView(next);
    }, []);
    const back = useCallback((restoreFocus: unknown = true) => {
        setView('grid');
        const was = opener.current;
        opener.current = null;
        if (restoreFocus === false || !was) {
            return;
        }

        // Once the grid is back. A grid drawn anew has a new button: the one of the same name.
        setTimeout(() => {
            const el = was.el.isConnected ? was.el : Array.from(was.root?.querySelectorAll<HTMLElement>('button[aria-label]') || []).find((b) => b.getAttribute('aria-label') === was.label);
            el?.focus();
        }, 0);
    }, []);
    return [view, open, back];
}

// ---------- The buttons next to the search box ----------

type ButtonsProps = {
    onAdd?: () => void;
    onManage?: () => void;
    addLabel: string;
    manageLabel: string;
};

// PickerViewButtons are the "+" (add) and manage buttons next to a tab's search box. Each one shows only with its
// handler, so that a tab passes them only to the people allowed to use them.
export function PickerViewButtons({onAdd, onManage, addLabel, manageLabel}: ButtonsProps) {
    return (
        <>
            {onAdd && (
                <button
                    type='button'
                    className={am('icon-btn', 'pv-btn')}
                    title={addLabel}
                    aria-label={addLabel}
                    onClick={onAdd}
                >
                    <Icon name='plus'/>
                </button>
            )}
            {onManage && (
                <button
                    type='button'
                    className={am('icon-btn', 'pv-btn')}
                    title={manageLabel}
                    aria-label={manageLabel}
                    onClick={onManage}
                >
                    <Icon name='library'/>
                </button>
            )}
        </>
    );
}

// ---------- The frame of a view ----------

type FrameProps = {
    title: string;
    onBack: () => void;

    // What the header has at its end (e.g. the page's "Add" button).
    tools?: React.ReactNode;

    // The buttons at the bottom (e.g. Save), always in sight.
    actions?: React.ReactNode;

    // 'page': the same view on a page of its own (the /:team/emoji page) rather than in a picker.
    variant?: 'picker' | 'page';

    // Without a back arrow: the page's first view.
    noBack?: boolean;
    children: React.ReactNode;
    onSubmit?: () => void;
};

// PickerViewFrame is a view's header (back arrow and title), its scrolling body and its actions. Escape goes back,
// and the focus moves into the view, unless a field of it took it already.
export function PickerViewFrame({title, onBack, tools, actions, variant = 'picker', noBack = false, children, onSubmit}: FrameProps) {
    const {formatMessage} = useIntl();
    const ref = useRef<HTMLElement>(null);
    const backRef = useRef<HTMLButtonElement>(null);
    const headingId = useId();
    useOnEscape(!noBack, onBack);

    useEffect(() => {
        const root = ref.current;
        if (root && !root.contains(root.ownerDocument.activeElement)) {
            (backRef.current || root.querySelector<HTMLElement>('input, button'))?.focus();
        }
    }, []);

    const backLabel = formatMessage({id: 'fusion.pickerView.back', defaultMessage: 'Back'});
    const body = (
        <>
            <div className={am('pv-head')}>
                {!noBack && (
                    <button
                        ref={backRef}
                        type='button'
                        className={am('icon-btn', 'pv-back')}
                        title={backLabel}
                        aria-label={backLabel}
                        onClick={onBack}
                    >
                        <Icon name='chev'/>
                    </button>
                )}
                <h3 id={headingId}>{title}</h3>
                {tools}
            </div>
            <div className={am('pv-body')}>{children}</div>
            {actions && <div className={am('pv-actions')}>{actions}</div>}
        </>
    );

    const className = am('pv', {'pv-page': variant === 'page'});
    if (onSubmit) {
        return (
            <form
                ref={ref as React.RefObject<HTMLFormElement>}
                className={className}
                aria-labelledby={headingId}
                noValidate={true}
                onSubmit={(e) => {
                    e.preventDefault();
                    onSubmit();
                }}
            >
                {body}
            </form>
        );
    }
    return (
        <section
            ref={ref}
            className={className}
            aria-labelledby={headingId}
        >
            {body}
        </section>
    );
}

// ---------- Messages ----------

// PickerNotice is an inline error (announced at once) or a status line (announced politely).
export function PickerNotice({kind = 'error', children}: {kind?: 'error' | 'ok' | 'info'; children: React.ReactNode}) {
    if (!children) {
        return null;
    }
    return (
        <p
            className={am('pv-notice', `pv-${kind}`)}
            role={kind === 'error' ? 'alert' : 'status'}
        >
            {children}
        </p>
    );
}

// ---------- Fields ----------

type FieldProps = {
    label: string;

    // What the field takes, under it; replaced by the error when there's one.
    hint?: React.ReactNode;
    error?: React.ReactNode;

    // A good outcome of a check, e.g. "the name is free".
    ok?: React.ReactNode;

    // At the end of the label, e.g. a character count.
    aside?: React.ReactNode;

    // The field: it gets the id and the description of the label and messages.
    children: (ids: {id: string; describedBy: string; invalid: boolean}) => React.ReactNode;
};

// PickerField is a labelled field with live feedback: its hint, or its error, or its good news, all read out by
// screen readers as the field's description.
export function PickerField({label, hint, error, ok, aside, children}: FieldProps) {
    const id = useId();
    const messageId = `${id}-message`;
    let message: React.ReactNode = hint;
    let kind = 'hint';
    if (error) {
        message = error;
        kind = 'error';
    } else if (ok) {
        message = ok;
        kind = 'ok';
    }
    return (
        <div className={am('field', 'pv-field', {invalid: Boolean(error)})}>
            <div className={am('pv-field-head')}>
                <label htmlFor={id}>{label}</label>
                {aside && <span className={am('pv-aside')}>{aside}</span>}
            </div>
            {children({id, describedBy: messageId, invalid: Boolean(error)})}
            <p
                id={messageId}
                className={am('pv-message', `pv-${kind}`)}
                aria-live='polite'
            >
                {message}
            </p>
        </div>
    );
}

// ---------- Search ----------

export function PickerSearch({value, onChange, placeholder, autoFocus = true}: {value: string; onChange: (value: string) => void; placeholder: string; autoFocus?: boolean}) {
    const ref = useRef<HTMLInputElement>(null);
    // Not on a touch screen, where it would bring the keyboard over half the picker: the field is a tap away.
    useEffect(() => {
        if (autoFocus && !isTouch()) {
            ref.current?.focus();
        }
    }, [autoFocus]);
    return (
        <div className={am('picker-search', 'pv-search')}>
            <label>
                <Icon
                    name='search'
                    size='sm'
                />
                <input
                    ref={ref}
                    type='search'
                    value={value}
                    placeholder={placeholder}
                    aria-label={placeholder}
                    autoComplete='off'
                    onChange={(e) => onChange(e.target.value)}
                    onKeyDown={(e) => {
                        // Down goes to the list under it.
                        if (e.key === 'ArrowDown') {
                            const first = e.currentTarget.closest('.am-pv')?.querySelector<HTMLElement>('[data-pv-row]');
                            if (first) {
                                e.preventDefault();
                                first.focus();
                            }
                        }
                    }}
                />
            </label>
        </div>
    );
}

// ---------- Images: choose, drop or paste ----------

type DropZoneProps = {

    // For the file chooser, e.g. 'image/png,image/gif,.png,.gif'.
    accept: string;
    file: File | null;
    onFile: (file: File) => void;

    // The zone's text, e.g. "Drop an image, paste one or choose a file".
    label: string;

    // The types and limits, under the zone.
    hint: React.ReactNode;
    error?: React.ReactNode;

    // The preview of the chosen file (e.g. the emoji at the sizes it shows at); without one, the image itself.
    preview?: React.ReactNode;

    // What the "choose" button says once there's a file.
    changeLabel: string;
};

// isImageFile tells whether a pasted or dropped file is an image the zone should take.
const firstFile = (list: FileList | null | undefined) => (list && list.length ? list[0] : null);

// PickerDropZone takes a file chosen with the file chooser, dropped on it, or pasted anywhere in its view (Ctrl/⌘+V
// with an image on the clipboard; pasting text in a field stays pasting text).
export function PickerDropZone({accept, file, onFile, label, hint, error, preview, changeLabel}: DropZoneProps) {
    const inputRef = useRef<HTMLInputElement>(null);
    const zoneRef = useRef<HTMLDivElement>(null);
    const [over, setOver] = useState(false);
    const hintId = useId();
    const onFileRef = useRef(onFile);
    onFileRef.current = onFile;

    const url = useMemo(() => (file && !preview ? URL.createObjectURL(file) : ''), [file, preview]);
    useEffect(() => () => {
        if (url) {
            URL.revokeObjectURL(url);
        }
    }, [url]);

    // A paste in the view, or anywhere while nothing else has the focus.
    useEffect(() => {
        const zone = zoneRef.current;
        const doc = zone?.ownerDocument || document;
        const onPaste = (e: ClipboardEvent) => {
            const target = e.target as Node | null;
            const view = zone?.closest('.am-pv');
            if (!view || (target && target !== doc.body && !view.contains(target))) {
                return;
            }
            const pasted = firstFile(e.clipboardData?.files);
            if (pasted) {
                e.preventDefault();
                onFileRef.current(pasted);
            }
        };
        doc.addEventListener('paste', onPaste);
        return () => doc.removeEventListener('paste', onPaste);
    }, []);

    return (
        <div className={am('pv-drop-wrap')}>
            <div
                ref={zoneRef}
                className={am('pv-drop', {over, filled: Boolean(file), invalid: Boolean(error)})}
                onDragEnter={(e) => {
                    e.preventDefault();
                    setOver(true);
                }}
                onDragOver={(e) => {
                    e.preventDefault();
                    e.dataTransfer.dropEffect = 'copy';
                    setOver(true);
                }}
                onDragLeave={(e) => {
                    if (!e.currentTarget.contains(e.relatedTarget as Node | null)) {
                        setOver(false);
                    }
                }}
                onDrop={(e) => {
                    e.preventDefault();
                    setOver(false);
                    const dropped = firstFile(e.dataTransfer?.files);
                    if (dropped) {
                        onFile(dropped);
                    }
                }}
            >
                {file && (
                    <div
                        className={am('pv-drop-preview')}
                        aria-hidden={!preview}
                    >
                        {preview || (
                            <img
                                src={url}
                                alt=''
                            />
                        )}
                    </div>
                )}
                {!file && (
                    <Icon
                        name='upload'
                        className={am('pv-drop-ic')}
                    />
                )}
                <input
                    ref={inputRef}
                    type='file'
                    accept={accept}
                    hidden={true}
                    onChange={(e) => {
                        const chosen = firstFile(e.target.files);
                        if (chosen) {
                            onFile(chosen);
                        }

                        // Choosing the same file again still counts.
                        e.target.value = '';
                    }}
                />
                <button
                    type='button'
                    className={am('btn', 'pv-drop-btn')}
                    aria-describedby={hintId}
                    onClick={() => inputRef.current?.click()}
                >
                    {file ? changeLabel : label}
                </button>
                {file && <span className={am('pv-drop-name')}>{file.name}</span>}
            </div>
            <p
                id={hintId}
                className={am('pv-message', error ? 'pv-error' : 'pv-hint')}
                role={error ? 'alert' : undefined}
            >
                {error || hint}
            </p>
        </div>
    );
}

// ---------- Lists of what was added ----------

export type PickerListItem = {
    id: string;

    // What the row says, e.g. ":party_parrot:".
    name: string;

    // Its picture.
    image: React.ReactNode;

    // Who added it and when (ms since the epoch); either may be unknown.
    by?: string;
    at?: number;

    // Whether the person may delete it: without, no delete button.
    canDelete: boolean;
};

type ListProps = {
    items: PickerListItem[];

    // Deletes an item; resolves to an error message when it couldn't.
    onDelete: (item: PickerListItem) => Promise<string | void>;

    // What the confirmation says, e.g. "Delete :x:? Messages that use it show :x: instead."
    confirmText: (item: PickerListItem) => string;
    deleteLabel: (item: PickerListItem) => string;
    deletedText: (item: PickerListItem) => string;

    // What the confirmation's button says, when not "Delete" (e.g. "Remove" for a favourite).
    confirmButton?: string;

    // When nothing is listed (yet, or for the search).
    empty: string;
    loading?: boolean;

    // More items to load at the end: the list asks as its end scrolls into view, and has a button for it too.
    hasMore?: boolean;
    onLoadMore?: () => void;

    // Picking a row (e.g. to use it); without, rows aren't buttons.
    onOpen?: (item: PickerListItem) => void;
};

// The rows drawn at first and added at a time as the list scrolls: thousands of custom emoji stay quick.
export const LIST_CHUNK = 60;

// PickerManageList lists what was added, with who added it and when, and a delete button where allowed. Deleting
// asks first, in the row. Long lists are drawn a chunk at a time as they scroll, and ask for more from the server.
// Up and Down move between the rows.
export function PickerManageList({items, onDelete, confirmText, deleteLabel, deletedText, confirmButton, empty, loading, hasMore, onLoadMore, onOpen}: ListProps) {
    const {formatMessage, formatDate} = useIntl();
    const [shown, setShown] = useState(LIST_CHUNK);
    const [confirming, setConfirming] = useState('');
    const [busy, setBusy] = useState('');
    const [error, setError] = useState('');
    const [done, setDone] = useState('');
    const endRef = useRef<HTMLDivElement>(null);
    const listRef = useRef<HTMLUListElement>(null);
    const more = items.length > shown || Boolean(hasMore);
    const moreRef = useRef(() => {});
    moreRef.current = () => {
        if (items.length > shown) {
            setShown(shown + LIST_CHUNK);
        } else if (hasMore && !loading) {
            onLoadMore?.();
        }
    };

    useOnEscape(Boolean(confirming), () => setConfirming(''));

    // A new search starts at its top.
    const first = items[0]?.id;
    useEffect(() => {
        setShown(LIST_CHUNK);
    }, [first]);

    useEffect(() => {
        const end = endRef.current;
        if (!end || !more || typeof IntersectionObserver === 'undefined') {
            return undefined;
        }
        const observer = new IntersectionObserver((entries) => {
            if (entries.some((e) => e.isIntersecting)) {
                moreRef.current();
            }
        }, {root: end.closest('.am-pv-body'), rootMargin: '120px'});
        observer.observe(end);
        return () => observer.disconnect();
    }, [more, shown, items.length]);

    const remove = async (item: PickerListItem) => {
        setBusy(item.id);
        setError('');
        setDone('');
        const why = await onDelete(item);
        setBusy('');
        setConfirming('');
        if (why) {
            setError(why);
        } else {
            setDone(deletedText(item));
            listRef.current?.closest('.am-pv')?.querySelector<HTMLInputElement>('input[type="search"]')?.focus();
        }
    };

    const onKeyDown = (e: React.KeyboardEvent) => {
        if (e.key !== 'ArrowDown' && e.key !== 'ArrowUp') {
            return;
        }
        const rows = Array.from(listRef.current?.querySelectorAll<HTMLElement>('[data-pv-row]') || []);
        const at = rows.indexOf(e.target as HTMLElement);
        if (at < 0) {
            return;
        }
        e.preventDefault();
        const next = rows[at + (e.key === 'ArrowDown' ? 1 : -1)];
        next?.focus();
    };

    const visible = items.slice(0, shown);
    return (
        <>
            <PickerNotice>{error}</PickerNotice>
            <PickerNotice kind='ok'>{done}</PickerNotice>
            {/* eslint-disable-next-line jsx-a11y/no-noninteractive-element-interactions */}
            <ul
                ref={listRef}
                className={am('pv-list')}
                onKeyDown={onKeyDown}
            >
                {visible.map((item) => {
                    const meta = [item.by, item.at ? formatDate(item.at, {year: 'numeric', month: 'short', day: 'numeric'}) : ''].filter(Boolean).join(' · ');
                    const info = (
                        <>
                            <span className={am('pv-row-img')}>{item.image}</span>
                            <span className={am('pv-row-text')}>
                                <span className={am('pv-row-name')}>{item.name}</span>
                                {meta && <span className={am('pv-row-meta')}>{meta}</span>}
                            </span>
                        </>
                    );
                    return (
                        <li
                            key={item.id}
                            className={am('pv-row', {confirming: confirming === item.id})}
                        >
                            {onOpen ? (
                                <button
                                    type='button'
                                    className={am('pv-row-main')}
                                    data-pv-row={true}
                                    onClick={() => onOpen(item)}
                                >
                                    {info}
                                </button>
                            ) : (
                                <span
                                    className={am('pv-row-main')}
                                    data-pv-row={true}
                                    tabIndex={-1}
                                >
                                    {info}
                                </span>
                            )}
                            {item.canDelete && confirming !== item.id && (
                                <button
                                    type='button'
                                    className={am('icon-btn', 'pv-row-delete')}
                                    title={deleteLabel(item)}
                                    aria-label={deleteLabel(item)}
                                    onClick={() => {
                                        setError('');
                                        setDone('');
                                        setConfirming(item.id);
                                    }}
                                >
                                    <Icon
                                        name='trash'
                                        size='sm'
                                    />
                                </button>
                            )}
                            {confirming === item.id && (
                                <ConfirmRow
                                    text={confirmText(item)}
                                    button={confirmButton}
                                    busy={busy === item.id}
                                    onKeep={() => setConfirming('')}
                                    onDelete={() => remove(item)}
                                />
                            )}
                        </li>
                    );
                })}
            </ul>
            {!items.length && !loading && <p className={am('pv-empty')}>{empty}</p>}
            {loading && <p className={am('pv-empty')}>{formatMessage({id: 'fusion.pickerView.loading', defaultMessage: 'Loading…'})}</p>}
            {more && !loading && (
                <div
                    ref={endRef}
                    className={am('pv-more')}
                >
                    <button
                        type='button'
                        className={am('btn', 'small')}
                        onClick={() => moreRef.current()}
                    >
                        {formatMessage({id: 'fusion.pickerView.more', defaultMessage: 'Show more'})}
                    </button>
                </div>
            )}
        </>
    );
}

// ConfirmRow asks, in the row, before deleting: Keep has the focus, so that Enter doesn't delete by mistake.
function ConfirmRow({text, button, busy, onKeep, onDelete}: {text: string; button?: string; busy: boolean; onKeep: () => void; onDelete: () => void}) {
    const {formatMessage} = useIntl();
    const keepRef = useRef<HTMLButtonElement>(null);
    useEffect(() => {
        keepRef.current?.focus();
    }, []);
    return (
        <div
            className={am('pv-confirm')}
            role='group'
            aria-label={text}
        >
            <span className={am('pv-confirm-text')}>{text}</span>
            <span className={am('pv-confirm-btns')}>
                <button
                    ref={keepRef}
                    type='button'
                    className={am('btn', 'small')}
                    onClick={onKeep}
                >
                    {formatMessage({id: 'fusion.pickerView.keep', defaultMessage: 'Keep'})}
                </button>
                <button
                    type='button'
                    className={am('btn', 'small', 'danger')}
                    disabled={busy}
                    onClick={onDelete}
                >
                    {busy ? formatMessage({id: 'fusion.pickerView.deleting', defaultMessage: 'Deleting…'}) : (button || formatMessage({id: 'fusion.pickerView.delete', defaultMessage: 'Delete'}))}
                </button>
            </span>
        </div>
    );
}

// ---------- For the plugins' tabs ----------

// The kit the picker hands to the plugins' tabs (EmojiPickerTabProps.pickerKit), so that their add and manage views
// look and behave like the emoji tab's. Plugins check for it: hosts without it keep their own dialogs.
export const pickerKit = {
    version: 1,
    usePickerView,
    useOnEscape,
    PickerViewButtons,
    PickerViewFrame,
    PickerNotice,
    PickerField,
    PickerSearch,
    PickerDropZone,
    PickerManageList,
};

export type PickerKit = typeof pickerKit;
