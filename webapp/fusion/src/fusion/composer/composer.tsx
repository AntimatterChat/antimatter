// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useEffect, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import type {FileInfo} from '@mattermost/types/files';
import type {PostPriority} from '@mattermost/types/posts';

import {Posts} from 'mattermost-redux/constants';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {isPostPriorityEnabled} from 'mattermost-redux/selectors/entities/posts';
import {getBool} from 'mattermost-redux/selectors/entities/preferences';
import {isScheduledPostsEnabled} from 'mattermost-redux/selectors/entities/scheduled_posts';

import {uploadFile} from 'actions/file_actions';
import {editLatestPost, onSubmit} from 'actions/views/create_comment';
import {updateDraft} from 'actions/views/drafts';
import {isBurnOnReadEnabled} from 'selectors/burn_on_read';
import {makeGetDraft} from 'selectors/drafts';

import Icon from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {MenuHeading, MenuItem, MenuSeparator} from 'fusion/components/menu';
import {useBurnDuration} from 'fusion/messages/burn_on_read';
import EmojiPicker from 'fusion/popovers/emoji_picker';
import {MENTION_EVENT} from 'fusion/popovers/user_menu';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import Constants, {StoragePrefixes} from 'utils/constants';
import {generateId} from 'utils/utils';

import type {GlobalState} from 'types/store';
import type {PostDraft} from 'types/store/draft';

import Autocomplete from './autocomplete';
import type {AutocompleteHandle} from './autocomplete';
import {FORMATS, applyFormat} from './formats';
import {useShowFormatting} from './formatting_preference';
import {SchedulePopover, ScheduledNote} from './schedule';
import SleepNote from './sleep_note';

type Props = {
    channelId: string;

    // Replies in a thread have a root; messages to the channel don't.
    rootId?: string;
    placeholder: string;

};

type Pending = {clientId: string; name: string; progress: number};

// Composer writes messages: the mockup's compose box around the classic web app's drafts and submit logic
// (slash commands, reactions, message priority).
export default function Composer({channelId, rootId = '', placeholder}: Props) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const dispatch = useDispatch();
    const getDraft = useMemo(() => makeGetDraft(), []);
    const storedDraft = useSelector((state: GlobalState) => getDraft(state, channelId, rootId));
    const priorityEnabled = useSelector(isPostPriorityEnabled);
    const burnEnabled = useSelector(isBurnOnReadEnabled);
    const schedulingEnabled = useSelector(isScheduledPostsEnabled);
    const burnDuration = useBurnDuration();
    const persistentMinutes = useSelector((state: GlobalState) => parseInt(getConfig(state).PersistentNotificationIntervalMinutes || '5', 10) || 5);
    const toast = useToast();
    const ctrlSend = useSelector((state: GlobalState) => getBool(state, Constants.Preferences.CATEGORY_ADVANCED_SETTINGS, 'send_on_ctrl_enter', false));
    const [draft, setDraft] = useState<PostDraft>(storedDraft);
    const [showFormatting, setShowFormatting] = useShowFormatting();
    const [pending, setPending] = useState<Pending[]>([]);
    const [menu, setMenu] = useState<'plus' | 'emoji' | 'priority' | 'burn' | 'more' | 'schedule' | null>(null);
    const textareaRef = useRef<HTMLTextAreaElement>(null);
    const fileRef = useRef<HTMLInputElement>(null);
    const plusRef = useRef<HTMLButtonElement>(null);
    const emojiRef = useRef<HTMLButtonElement>(null);
    const priorityRef = useRef<HTMLButtonElement>(null);
    const burnRef = useRef<HTMLButtonElement>(null);
    const moreRef = useRef<HTMLButtonElement>(null);
    const autocompleteRef = useRef<AutocompleteHandle>(null);
    const saveTimer = useRef<ReturnType<typeof setTimeout>>(undefined);

    const key = rootId ? StoragePrefixes.COMMENT_DRAFT + rootId : StoragePrefixes.DRAFT + channelId;

    // Switch drafts with the conversation.
    useEffect(() => {
        setDraft(storedDraft);
        setPending([]);

        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [channelId, rootId]);

    const grow = useCallback(() => {
        const el = textareaRef.current;
        if (el) {
            el.style.height = 'auto';
            el.style.height = Math.min(el.scrollHeight, 220) + 'px';
        }
    }, []);
    useEffect(grow, [draft.message, grow]);

    const save = useCallback((next: PostDraft) => {
        setDraft(next);
        clearTimeout(saveTimer.current);
        saveTimer.current = setTimeout(() => dispatch(updateDraft(key, {...next, updateAt: Date.now()}, rootId, true)), 500);
    }, [dispatch, key, rootId]);

    const change = (patch: Partial<PostDraft>) => save({...draft, channelId, rootId, createAt: draft.createAt || Date.now(), ...patch});

    const submit = async () => {
        const message = draft.message;
        if (!message.trim() && !draft.fileInfos.length) {
            return;
        }
        if (pending.length) {
            return;
        }
        const toSend: PostDraft = {...draft, channelId, rootId};
        const empty: PostDraft = {message: '', fileInfos: [], uploadsInProgress: [], channelId, rootId, createAt: 0, updateAt: 0};
        setDraft(empty);
        clearTimeout(saveTimer.current);
        dispatch(updateDraft(key, null, rootId, true));
        const result = await dispatch(onSubmit(channelId, rootId, toSend, {}));
        if (result && 'error' in result && result.error) {
            setDraft(toSend);
        }
    };

    // Scheduling sends the draft later instead of now (Mattermost's scheduled messages).
    const schedule = async (at: number, sleeper?: string) => {
        setMenu(null);
        if (!draft.message.trim() && !draft.fileInfos.length) {
            toast(formatMessage({id: 'fusion.toast.scheduleEmpty', defaultMessage: 'Write your message first, then schedule it'}));
            textareaRef.current?.focus();
            return;
        }
        if (pending.length) {
            return;
        }
        const toSend: PostDraft = {...draft, channelId, rootId};
        const empty: PostDraft = {message: '', fileInfos: [], uploadsInProgress: [], channelId, rootId, createAt: 0, updateAt: 0};
        setDraft(empty);
        clearTimeout(saveTimer.current);
        dispatch(updateDraft(key, null, rootId, true));
        const result = await dispatch(onSubmit(channelId, rootId, toSend, {}, {scheduled_at: at}));
        if (result && 'error' in result && result.error) {
            setDraft(toSend);
            toast(formatMessage({id: 'fusion.toast.scheduleFailed', defaultMessage: 'The message could not be scheduled'}));
            return;
        }
        const when = intl.formatDate(at, {weekday: 'short', hour: 'numeric', minute: '2-digit'});
        if (sleeper) {
            const hours = Math.max(1, Math.round((at - Date.now()) / 36e5));
            toast(formatMessage({id: 'fusion.toast.scheduledSleeper', defaultMessage: 'Scheduled — {name} gets it at 07:00 their time (in about {hours} h)'}, {name: sleeper, hours}));
        } else {
            toast(formatMessage({id: 'fusion.toast.scheduled', defaultMessage: 'Scheduled for {when}'}, {when}));
        }
    };

    const onKeyDown = (e: React.KeyboardEvent<HTMLTextAreaElement>) => {
        if (autocompleteRef.current?.handleKeyDown(e)) {
            return;
        }

        // Up in an empty composer edits your last message, as in the classic web app.
        if (e.key === 'ArrowUp' && !draft.message && !e.shiftKey && !e.altKey && !e.ctrlKey && !e.metaKey) {
            e.preventDefault();
            dispatch(editLatestPost(channelId, rootId));
            return;
        }
        if (e.key === 'Enter' && !e.nativeEvent.isComposing) {
            const withCtrl = e.ctrlKey || e.metaKey;
            if ((ctrlSend && withCtrl) || (!ctrlSend && !e.shiftKey && !withCtrl && !e.altKey)) {
                e.preventDefault();
                submit();
            }
        }
    };

    const insert = (text: string) => {
        const el = textareaRef.current;
        const start = el?.selectionStart ?? draft.message.length;
        const end = el?.selectionEnd ?? draft.message.length;
        const value = draft.message.slice(0, start) + text + draft.message.slice(end);
        change({message: value});
        requestAnimationFrame(() => {
            el?.focus();
            el?.setSelectionRange(start + text.length, start + text.length);
        });
    };

    // "Mention" in a person's menu writes @username into the conversation's composer.
    const insertRef = useRef(insert);
    insertRef.current = insert;
    useEffect(() => {
        if (rootId) {
            return undefined;
        }
        const onMention = (e: Event) => insertRef.current((e as CustomEvent<string>).detail);
        window.addEventListener(MENTION_EVENT, onMention);
        return () => window.removeEventListener(MENTION_EVENT, onMention);
    }, [rootId]);

    const format = (id: string) => {
        const f = FORMATS.find((x) => x && x.id === id);
        const el = textareaRef.current;
        if (!f || !el) {
            return;
        }
        const next = applyFormat(draft.message, el.selectionStart, el.selectionEnd, f);
        change({message: next.value});
        requestAnimationFrame(() => {
            el.focus();
            el.setSelectionRange(next.start, next.end);
        });
    };

    const upload = (files: FileList | File[]) => {
        Array.from(files).forEach((file) => {
            const clientId = generateId();
            setPending((p) => [...p, {clientId, name: file.name, progress: 0}]);
            dispatch(uploadFile({
                file,
                name: file.name,
                type: file.type,
                rootId,
                channelId,
                clientId,
                onProgress: (info) => setPending((p) => p.map((x) => (x.clientId === clientId ? {...x, progress: info.percent || 0} : x))),
                onSuccess: (data: {file_infos: FileInfo[]}) => {
                    setPending((p) => p.filter((x) => x.clientId !== clientId));
                    setDraft((d) => {
                        const next = {...d, fileInfos: [...d.fileInfos, ...data.file_infos]};
                        dispatch(updateDraft(key, next, rootId, true));
                        return next;
                    });
                },
                onError: () => setPending((p) => p.filter((x) => x.clientId !== clientId)),
            }));
        });
    };

    const priority = draft.metadata?.priority;
    const setPriority = (patch: Partial<{priority: PostPriority | ''; requested_ack: boolean; persistent_notifications: boolean}>) => {
        const next = {priority: (priority?.priority || '') as PostPriority | '', requested_ack: priority?.requested_ack || false, persistent_notifications: priority?.persistent_notifications || false, ...patch};
        if (next.priority !== 'urgent') {
            next.persistent_notifications = false;
        }
        change({metadata: {...draft.metadata, priority: next}});
    };
    const burn = draft.type === Posts.POST_TYPES.BURN_ON_READ;

    const chips = [];
    if (priority?.priority) {
        chips.push(
            <span
                key='prio'
                className={am('opt-chip', priority.priority)}
            >
                <Icon name='flag'/>
                {priority.priority === 'urgent' ? formatMessage({id: 'fusion.composer.urgent', defaultMessage: 'Urgent'}) : formatMessage({id: 'fusion.composer.important', defaultMessage: 'Important'})}
                {priority.requested_ack ? formatMessage({id: 'fusion.composer.withAck', defaultMessage: ' · acknowledgement'}) : ''}
                {priority.persistent_notifications ? formatMessage({id: 'fusion.composer.persistent', defaultMessage: ' · persistent'}) : ''}
                <button
                    type='button'
                    aria-label={formatMessage({id: 'fusion.composer.removePriority', defaultMessage: 'Remove priority'})}
                    onClick={() => setPriority({priority: '', requested_ack: false})}
                >
                    <Icon name='x'/>
                </button>
            </span>,
        );
    } else if (priority?.requested_ack) {
        chips.push(
            <span
                key='ack'
                className={am('opt-chip')}
            >
                <Icon name='check'/>
                {formatMessage({id: 'fusion.composer.ackRequested', defaultMessage: 'Acknowledgement requested'})}
                <button
                    type='button'
                    aria-label={formatMessage({id: 'fusion.composer.removeAck', defaultMessage: 'Remove acknowledgement'})}
                    onClick={() => setPriority({requested_ack: false})}
                >
                    <Icon name='x'/>
                </button>
            </span>,
        );
    }
    if (burn) {
        chips.push(
            <span
                key='burn'
                className={am('opt-chip', 'burn')}
            >
                <Icon name='flame'/>
                {formatMessage({id: 'fusion.composer.burns', defaultMessage: 'Burns on read'})}
                <button
                    type='button'
                    aria-label={formatMessage({id: 'fusion.composer.noBurn', defaultMessage: 'Turn off burn on read'})}
                    onClick={() => change({type: undefined})}
                >
                    <Icon name='x'/>
                </button>
            </span>,
        );
    }
    const files = draft.fileInfos.map((f) => (
        <span
            key={f.id}
            className={am('opt-chip')}
        >
            <Icon name='attach'/>
            {f.name}
            <button
                type='button'
                aria-label={formatMessage({id: 'fusion.composer.removeFile', defaultMessage: 'Remove {name}'}, {name: f.name})}
                onClick={() => change({fileInfos: draft.fileInfos.filter((x) => x.id !== f.id)})}
            >
                <Icon name='x'/>
            </button>
        </span>
    ));
    const uploads = pending.map((p) => (
        <span
            key={p.clientId}
            className={am('opt-chip')}
        >
            <Icon name='upload'/>
            {`${p.name} · ${Math.round(p.progress)}%`}
        </span>
    ));

    let group = 0;
    return (
        <form
            className={am('composer')}
            onSubmit={(e) => {
                e.preventDefault();
                submit();
            }}
            onDragOver={(e) => e.preventDefault()}
            onDrop={(e) => {
                if (e.dataTransfer.files.length) {
                    e.preventDefault();
                    upload(e.dataTransfer.files);
                }
            }}
        >
            <Autocomplete
                ref={autocompleteRef}
                channelId={channelId}
                rootId={rootId}
                value={draft.message}
                textareaRef={textareaRef}
                onChange={(message) => change({message})}
            />
            {!rootId && (
                <SleepNote
                    channelId={channelId}
                    onSendAt={schedulingEnabled ? schedule : undefined}
                />
            )}
            {schedulingEnabled && <ScheduledNote id={rootId || channelId}/>}
            <div className={am('compose-box')}>
                {(chips.length > 0 || files.length > 0 || uploads.length > 0) && <div className={am('opt-chips')}>{chips}{files}{uploads}</div>}
                <div className={am('compose-row')}>
                    <button
                        ref={plusRef}
                        type='button'
                        className={am('plus-btn')}
                        aria-label={formatMessage({id: 'fusion.composer.plus', defaultMessage: 'Add a file and more'})}
                        aria-haspopup='menu'
                        aria-expanded={menu === 'plus'}
                        onClick={() => setMenu(menu === 'plus' ? null : 'plus')}
                    >
                        <span><Icon name='plus'/></span>
                    </button>
                    <textarea
                        ref={textareaRef}
                        rows={1}
                        value={draft.message}
                        placeholder={placeholder}
                        aria-label={placeholder}
                        onChange={(e) => change({message: e.target.value})}
                        onKeyDown={onKeyDown}
                        onPaste={(e) => {
                            if (e.clipboardData.files.length) {
                                e.preventDefault();
                                upload(e.clipboardData.files);
                            }
                        }}
                    />
                    <button
                        type='button'
                        className={am('icon-btn', {on: showFormatting})}
                        aria-pressed={showFormatting}
                        title={showFormatting ? formatMessage({id: 'fusion.composer.hideFormatting', defaultMessage: 'Hide formatting'}) : formatMessage({id: 'fusion.composer.showFormatting', defaultMessage: 'Show formatting'})}
                        aria-label={formatMessage({id: 'fusion.composer.showFormatting', defaultMessage: 'Show formatting'})}
                        onClick={() => setShowFormatting(!showFormatting)}
                    >
                        <Icon name='type'/>
                    </button>
                    <button
                        ref={emojiRef}
                        type='button'
                        className={am('icon-btn')}
                        title={formatMessage({id: 'fusion.composer.emoji', defaultMessage: 'Emoji'})}
                        aria-label={formatMessage({id: 'fusion.composer.emoji', defaultMessage: 'Emoji'})}
                        aria-haspopup='dialog'
                        onClick={() => setMenu(menu === 'emoji' ? null : 'emoji')}
                    >
                        <Icon name='smile'/>
                    </button>
                    <button
                        className={am('icon-btn', 'send-btn')}
                        aria-label={formatMessage({id: 'fusion.composer.send', defaultMessage: 'Send'})}
                        disabled={pending.length > 0}
                    >
                        <Icon
                            name='send'
                            size='sm'
                        />
                    </button>
                </div>
                {showFormatting && (
                    <div
                        className={am('fmt-bar')}
                        role='toolbar'
                        aria-label={formatMessage({id: 'fusion.composer.formatting', defaultMessage: 'Formatting and message options'})}
                    >
                        {FORMATS.map((f, i) => {
                            if (!f) {
                                group += 1;
                                return (
                                    <span
                                        key={`sep-${i}`}
                                        className={am('fmt-sep', `g${group}`)}
                                    />
                                );
                            }
                            return (
                                <button
                                    key={f.id}
                                    type='button'
                                    className={am(`g${group}`)}
                                    title={formatMessage(f.label)}
                                    aria-label={formatMessage(f.label)}
                                    onClick={() => format(f.id)}
                                >
                                    <Icon name={f.id}/>
                                </button>
                            );
                        })}
                        <button
                            ref={moreRef}
                            type='button'
                            className={am('fmt-more')}
                            title={formatMessage({id: 'fusion.composer.moreFormatting', defaultMessage: 'More formatting'})}
                            aria-label={formatMessage({id: 'fusion.composer.moreFormatting', defaultMessage: 'More formatting'})}
                            aria-haspopup='menu'
                            onClick={() => setMenu(menu === 'more' ? null : 'more')}
                        >
                            <Icon name='dots'/>
                        </button>
                        <span className={am('grow')}/>
                        {priorityEnabled && !rootId && (
                            <button
                                ref={priorityRef}
                                type='button'
                                className={am({on: Boolean(priority?.priority), [`prio-${priority?.priority}`]: Boolean(priority?.priority)})}
                                title={formatMessage({id: 'fusion.composer.priority', defaultMessage: 'Message priority'})}
                                aria-label={formatMessage({id: 'fusion.composer.priority', defaultMessage: 'Message priority'})}
                                aria-haspopup='menu'
                                onClick={() => setMenu(menu === 'priority' ? null : 'priority')}
                            >
                                <Icon name='flag'/>
                            </button>
                        )}
                        {burnEnabled && (
                            <button
                                ref={burnRef}
                                type='button'
                                className={am({on: burn, burn})}
                                title={formatMessage({id: 'fusion.composer.burn', defaultMessage: 'Burn on read'})}
                                aria-label={formatMessage({id: 'fusion.composer.burn', defaultMessage: 'Burn on read'})}
                                aria-haspopup='menu'
                                onClick={() => setMenu(menu === 'burn' ? null : 'burn')}
                            >
                                <Icon name='flame'/>
                            </button>
                        )}
                    </div>
                )}
            </div>
            <input
                ref={fileRef}
                type='file'
                multiple={true}
                hidden={true}
                onChange={(e) => {
                    if (e.target.files) {
                        upload(e.target.files);
                    }
                    e.target.value = '';
                }}
            />
            {menu === 'plus' && (
                <Popover
                    anchor={plusRef.current}
                    placement='above'
                    className='plus-pop'
                    role='menu'
                    label={formatMessage({id: 'fusion.composer.plusMenu', defaultMessage: 'Add to message'})}
                    onClose={() => setMenu(null)}
                >
                    <MenuItem
                        icon='upload'
                        label={formatMessage({id: 'fusion.composer.upload', defaultMessage: 'Upload a file'})}
                        onClick={() => {
                            setMenu(null);
                            fileRef.current?.click();
                        }}
                    />
                    <MenuSeparator/>
                    {schedulingEnabled && (
                        <MenuItem
                            icon='clock'
                            label={formatMessage({id: 'fusion.composer.schedule', defaultMessage: 'Schedule message'})}
                            onClick={() => setMenu('schedule')}
                        />
                    )}
                    <MenuItem
                        icon='slash'
                        label={formatMessage({id: 'fusion.composer.slash', defaultMessage: 'Use a slash command'})}
                        onClick={() => {
                            setMenu(null);
                            change({message: '/'});
                            requestAnimationFrame(() => textareaRef.current?.focus());
                        }}
                    />
                </Popover>
            )}
            {menu === 'schedule' && (
                <SchedulePopover
                    anchor={plusRef.current}
                    onSchedule={(at) => schedule(at)}
                    onClose={() => setMenu(null)}
                />
            )}
            {menu === 'emoji' && (
                <EmojiPicker
                    anchor={emojiRef.current}
                    keepOpen={true}
                    onPick={(name) => insert(`:${name}: `)}
                    onClose={() => setMenu(null)}
                />
            )}
            {menu === 'more' && (
                <Popover
                    anchor={moreRef.current}
                    placement='above'
                    className='plus-pop fmt-pop'
                    role='menu'
                    label={formatMessage({id: 'fusion.composer.moreFormatting', defaultMessage: 'More formatting'})}
                    onClose={() => setMenu(null)}
                >
                    <div className={am('fmt-grid')}>
                        {FORMATS.filter((f, i) => f && FORMATS.slice(0, i).filter((x) => x === null).length >= 2).map((f) => (
                            <button
                                key={f!.id}
                                type='button'
                                role='menuitem'
                                title={formatMessage(f!.label)}
                                aria-label={formatMessage(f!.label)}
                                onClick={() => format(f!.id)}
                            >
                                <Icon name={f!.id}/>
                            </button>
                        ))}
                    </div>
                </Popover>
            )}
            {menu === 'priority' && (
                <Popover
                    anchor={priorityRef.current}
                    placement='above'
                    className='plus-pop prio-pop'
                    role='menu'
                    label={formatMessage({id: 'fusion.composer.priority', defaultMessage: 'Message priority'})}
                    onClose={() => setMenu(null)}
                >
                    <MenuHeading>{formatMessage({id: 'fusion.composer.priority', defaultMessage: 'Message priority'})}</MenuHeading>
                    {(['', 'important', 'urgent'] as const).map((p) => (
                        <MenuItem
                            key={p || 'standard'}
                            role='menuitemradio'
                            checked={(priority?.priority || '') === p}
                            icon={p ? 'flag' : 'chat'}
                            label={p ? <span className={am('prio', p)}>{p === 'urgent' ? formatMessage({id: 'fusion.composer.urgent', defaultMessage: 'Urgent'}) : formatMessage({id: 'fusion.composer.important', defaultMessage: 'Important'})}</span> : formatMessage({id: 'fusion.composer.standard', defaultMessage: 'Standard'})}
                            onClick={() => setPriority({priority: p as PostPriority | ''})}
                        />
                    ))}
                    <MenuSeparator/>
                    <div className={am('pp-row')}>
                        <span>
                            {formatMessage({id: 'fusion.composer.requestAck', defaultMessage: 'Request acknowledgement'})}
                            <small>{formatMessage({id: 'fusion.composer.requestAckDesc', defaultMessage: 'Recipients confirm they have read it'})}</small>
                        </span>
                        <button
                            type='button'
                            className={am('switch')}
                            role='switch'
                            aria-checked={Boolean(priority?.requested_ack)}
                            aria-label={formatMessage({id: 'fusion.composer.requestAck', defaultMessage: 'Request acknowledgement'})}
                            onClick={() => setPriority({requested_ack: !priority?.requested_ack})}
                        />
                    </div>
                    <div className={am('pp-row', {disabled: priority?.priority !== 'urgent'})}>
                        <span>
                            {formatMessage({id: 'fusion.composer.persistentTitle', defaultMessage: 'Persistent notifications'})}
                            <small>{formatMessage({id: 'fusion.composer.persistentEvery', defaultMessage: 'Repeats every {minutes, plural, one {minute} other {# minutes}} until acknowledged · urgent only'}, {minutes: persistentMinutes})}</small>
                        </span>
                        <button
                            type='button'
                            className={am('switch')}
                            role='switch'
                            aria-checked={Boolean(priority?.persistent_notifications)}
                            aria-label={formatMessage({id: 'fusion.composer.persistentTitle', defaultMessage: 'Persistent notifications'})}
                            onClick={() => setPriority({persistent_notifications: !priority?.persistent_notifications})}
                        />
                    </div>
                    <div className={am('pp-foot')}>
                        <button
                            type='button'
                            className={am('btn', 'primary')}
                            onClick={() => setMenu(null)}
                        >
                            {formatMessage({id: 'fusion.composer.done', defaultMessage: 'Done'})}
                        </button>
                    </div>
                </Popover>
            )}
            {menu === 'burn' && (
                <Popover
                    anchor={burnRef.current}
                    placement='above'
                    className='plus-pop prio-pop'
                    role='menu'
                    label={formatMessage({id: 'fusion.composer.burn', defaultMessage: 'Burn on read'})}
                    onClose={() => setMenu(null)}
                >
                    <div className={am('pp-row')}>
                        <span>
                            {formatMessage({id: 'fusion.composer.burn', defaultMessage: 'Burn on read'})}
                            <small>{formatMessage({id: 'fusion.composer.burnAfter', defaultMessage: 'Deleted {duration} after each person opens it'}, {duration: burnDuration})}</small>
                        </span>
                        <button
                            type='button'
                            className={am('switch', 'burn')}
                            role='switch'
                            aria-checked={burn}
                            aria-label={formatMessage({id: 'fusion.composer.burn', defaultMessage: 'Burn on read'})}
                            onClick={() => change({type: burn ? undefined : Posts.POST_TYPES.BURN_ON_READ})}
                        />
                    </div>
                    <div className={am('pp-foot')}>
                        <button
                            type='button'
                            className={am('btn', 'primary')}
                            onClick={() => setMenu(null)}
                        >
                            {formatMessage({id: 'fusion.composer.done', defaultMessage: 'Done'})}
                        </button>
                    </div>
                </Popover>
            )}
        </form>
    );
}
