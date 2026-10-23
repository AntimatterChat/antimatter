// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import type {IntlShape} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {CustomStatusDuration} from '@mattermost/types/users';
import type {UserCustomStatus} from '@mattermost/types/users';

import {removeRecentCustomStatus, setCustomStatus, setStatus, unsetCustomStatus, updateMe} from 'mattermost-redux/actions/users';
import {Client4} from 'mattermost-redux/client';
import {getConfig} from 'mattermost-redux/selectors/entities/general';
import {getCurrentUser, getDndEndTimeForUserId} from 'mattermost-redux/selectors/entities/users';

import {emitUserLoggedOutEvent} from 'actions/global_actions';
import {openModal} from 'actions/views/modals';
import {getRecentCustomStatuses, makeGetCustomStatus} from 'selectors/views/custom_status';

import RenderEmoji from 'components/emoji/render_emoji';
import MarketplaceModal from 'components/plugin_marketplace/marketplace_modal';

import Avatar from 'fusion/components/avatar';
import Icon from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {MenuHeading, MenuItem} from 'fusion/components/menu';
import StatusEmoji from 'fusion/components/status_emoji';
import {CustomTimeForm} from 'fusion/composer/schedule';
import {useFocusLabel, useFocusTime, useMyFocusUntil} from 'fusion/focus/focus';
import {useDisplayName, useUserStatus} from 'fusion/hooks/users';
import EmojiPicker from 'fusion/popovers/emoji_picker';
import {useDialogs} from 'fusion/shell/dialogs_context';
import {useSettings} from 'fusion/shell/settings_context';
import {useToast} from 'fusion/shell/toast_context';
import {am} from 'fusion/utils/class_names';
import {getHistory} from 'utils/browser_history';
import {ModalIdentifiers} from 'utils/constants';

import type {GlobalState} from 'types/store';

import {canOpenMarketplace, canVisitSystemConsole} from './admin_links';

type Props = {
    anchor: HTMLElement | null;
    onClose: () => void;
};

const STATUSES = [
    {status: 'online', label: {id: 'fusion.status.online', defaultMessage: 'Online'}, sub: null},
    {status: 'away', label: {id: 'fusion.status.away', defaultMessage: 'Away'}, sub: null},
    {status: 'dnd', label: {id: 'fusion.status.dnd', defaultMessage: 'Do not disturb'}, sub: {id: 'fusion.status.dndSub', defaultMessage: 'Mutes notifications'}},
    {status: 'offline', label: {id: 'fusion.status.invisible', defaultMessage: 'Invisible'}, sub: {id: 'fusion.status.invisibleSub', defaultMessage: 'You appear offline but can still use Antimatter'}},
];
const OUT_OF_OFFICE = {status: 'ooo', label: {id: 'fusion.status.ooo', defaultMessage: 'Out of office'}, sub: {id: 'fusion.status.oooSub', defaultMessage: 'Automatic replies are on'}};

// Suggested custom statuses: Mattermost's, and the out of office ones that can turn on automatic replies.
const SUGGESTIONS: Array<{emoji: string; text: {id: string; defaultMessage: string}; duration: CustomStatusDuration; away?: boolean}> = [
    {emoji: 'calendar', text: {id: 'fusion.status.inMeeting', defaultMessage: 'In a meeting'}, duration: CustomStatusDuration.ONE_HOUR},
    {emoji: 'hamburger', text: {id: 'fusion.status.lunch', defaultMessage: 'Out for lunch'}, duration: CustomStatusDuration.THIRTY_MINUTES},
    {emoji: 'house', text: {id: 'fusion.status.remote', defaultMessage: 'Remote working'}, duration: CustomStatusDuration.TODAY},
    {emoji: 'sneezing_face', text: {id: 'fusion.status.sick', defaultMessage: 'Out sick'}, duration: CustomStatusDuration.TODAY, away: true},
    {emoji: 'door', text: {id: 'fusion.status.outOfOffice', defaultMessage: 'Out of office'}, duration: CustomStatusDuration.TODAY, away: true},
    {emoji: 'palm_tree', text: {id: 'fusion.status.vacation', defaultMessage: 'On vacation'}, duration: CustomStatusDuration.THIS_WEEK, away: true},
];

// Do not disturb durations, as in the mockup. The end time is in seconds; 0 means until turned off.
const DND_DURATIONS: Array<[string, number]> = [
    ['15min', 15 * 6e4],
    ['1h', 36e5],
    ['1d', 864e5],
    ['1w', 7 * 864e5],
    ['forever', 0],
];
const dndEnd = (ms: number) => (ms ? Math.floor((Date.now() + ms) / 1000) : 0);

// dndDurationLabel is a DND_DURATIONS key's name: a literal call per key, not a lookup into a stored message
// descriptor, so every one of them is extracted (a map, or an array of tuples holding the descriptor, isn't: these
// ids were missing from en.json, never translated, until this fix).
function dndDurationLabel(formatMessage: IntlShape['formatMessage'], key: string): string {
    switch (key) {
    case '1h': return formatMessage({id: 'fusion.status.for1h', defaultMessage: 'For 1 hour'});
    case '1d': return formatMessage({id: 'fusion.status.for1d', defaultMessage: 'For 1 day'});
    case '1w': return formatMessage({id: 'fusion.status.for1w', defaultMessage: 'For 1 week'});
    case 'forever': return formatMessage({id: 'fusion.status.forever', defaultMessage: 'Forever'});
    default: return formatMessage({id: 'fusion.status.for15', defaultMessage: 'For 15 minutes'});
    }
}

const CLEAR_AFTER: CustomStatusDuration[] = [
    CustomStatusDuration.THIRTY_MINUTES,
    CustomStatusDuration.ONE_HOUR,
    CustomStatusDuration.FOUR_HOURS,
    CustomStatusDuration.TODAY,
    CustomStatusDuration.THIS_WEEK,
    CustomStatusDuration.DONT_CLEAR,
];

// clearAfterLabel is a CLEAR_AFTER choice's name, the same way: a literal call per choice. Anything not listed
// (DATE_AND_TIME has its own label, built from the date and time chosen) falls back to Today's, as the custom
// status input's own duration label already did.
function clearAfterLabel(formatMessage: IntlShape['formatMessage'], duration: CustomStatusDuration): string {
    switch (duration) {
    case CustomStatusDuration.THIRTY_MINUTES: return formatMessage({id: 'fusion.status.clear30', defaultMessage: '30 minutes'});
    case CustomStatusDuration.ONE_HOUR: return formatMessage({id: 'fusion.status.clear1h', defaultMessage: '1 hour'});
    case CustomStatusDuration.FOUR_HOURS: return formatMessage({id: 'fusion.status.clear4h', defaultMessage: '4 hours'});
    case CustomStatusDuration.THIS_WEEK: return formatMessage({id: 'fusion.status.clearWeek', defaultMessage: 'This week'});
    case CustomStatusDuration.DONT_CLEAR: return formatMessage({id: 'fusion.status.clearNever', defaultMessage: "Don't clear"});
    default: return formatMessage({id: 'fusion.status.clearToday', defaultMessage: 'Today'});
    }
}

function expiresAt(duration: CustomStatusDuration): string | undefined {
    const now = new Date();
    switch (duration) {
    case CustomStatusDuration.THIRTY_MINUTES: return new Date(now.getTime() + (30 * 6e4)).toISOString();
    case CustomStatusDuration.ONE_HOUR: return new Date(now.getTime() + 36e5).toISOString();
    case CustomStatusDuration.FOUR_HOURS: return new Date(now.getTime() + (4 * 36e5)).toISOString();
    case CustomStatusDuration.TODAY: {
        const end = new Date(now);
        end.setHours(23, 59, 59, 999);
        return end.toISOString();
    }
    case CustomStatusDuration.THIS_WEEK: {
        const end = new Date(now);
        end.setDate(end.getDate() + ((7 - end.getDay()) % 7));
        end.setHours(23, 59, 59, 999);
        return end.toISOString();
    }
    default: return undefined;
    }
}

// StatusPopover sets your status and custom status, and leads to your settings, from your avatar.
export default function StatusPopover({anchor, onClose}: Props) {
    const intl = useIntl();
    const {formatMessage} = intl;
    const dispatch = useDispatch();
    const settings = useSettings();
    const dialogs = useDialogs();
    const toast = useToast();
    const me = useSelector(getCurrentUser);
    const name = useDisplayName(me);
    const status = useUserStatus(me?.id);

    // Focus mode: Do not disturb until a time, started and ended here (and nowhere else).
    const focusUntil = useMyFocusUntil();
    const focusLabel = useFocusLabel(focusUntil);
    const focusTime = useFocusTime(focusUntil);
    const dndEndTime = useSelector((state: GlobalState) => (me ? getDndEndTimeForUserId(state, me.id) : 0));
    const getCustomStatus = useMemo(() => makeGetCustomStatus(), []);
    const custom = useSelector((state: GlobalState) => getCustomStatus(state, me?.id));
    const [text, setText] = useState(custom?.text || '');
    const [emoji, setEmoji] = useState(custom?.emoji || '');
    const [picking, setPicking] = useState(false);
    const emojiButton = useRef<HTMLButtonElement>(null);
    const [choosingDuration, setChoosingDuration] = useState(false);
    const [dndCustom, setDndCustom] = useState(false);
    const clockButton = useRef<HTMLButtonElement>(null);
    const [duration, setDuration] = useState<CustomStatusDuration>(custom?.duration || CustomStatusDuration.TODAY);
    const [clearAt, setClearAt] = useState(custom?.duration === CustomStatusDuration.DATE_AND_TIME ? custom.expires_at : undefined);
    const [clearCustom, setClearCustom] = useState(false);
    const [focused, setFocused] = useState(false);
    const [typed, setTyped] = useState(false);
    const [offerReplies, setOfferReplies] = useState(false);
    const recents = useSelector(getRecentCustomStatuses);
    const autoReplies = useSelector((state: GlobalState) => getConfig(state).ExperimentalEnableAutomaticReplies === 'true');
    const systemConsole = useSelector(canVisitSystemConsole);
    const marketplace = useSelector(canOpenMarketplace);
    const durationLabel = duration === CustomStatusDuration.DATE_AND_TIME && clearAt ? intl.formatDate(clearAt, {weekday: 'short', day: 'numeric', month: 'short', hour: 'numeric', minute: '2-digit'}) : clearAfterLabel(formatMessage, duration);

    if (!me) {
        return null;
    }
    const label = (s: string) => formatMessage([...STATUSES, OUT_OF_OFFICE].find((x) => x.status === s)?.label || STATUSES[3].label);

    const saveCustom = (e?: React.FormEvent, withEmoji = emoji) => {
        e?.preventDefault();
        if (text.trim() === (custom?.text || '') && withEmoji === (custom?.emoji || '')) {
            return;
        }
        if (text.trim() || withEmoji) {
            dispatch(setCustomStatus({emoji: withEmoji || 'speech_balloon', text: text.trim(), duration, expires_at: duration === CustomStatusDuration.DATE_AND_TIME ? clearAt : expiresAt(duration)}));
            toast(formatMessage({id: 'fusion.toast.customStatus', defaultMessage: 'Custom status updated'}));
        } else {
            dispatch(unsetCustomStatus());
            toast(formatMessage({id: 'fusion.toast.customStatusCleared', defaultMessage: 'Custom status cleared'}));
        }
    };

    // applyStatus sets a suggested or recent custom status as it is, and for the out of office ones offers to turn on the
    // automatic replies.
    const applyStatus = (next: UserCustomStatus, away = false) => {
        setText(next.text);
        setEmoji(next.emoji);
        setDuration(next.duration || CustomStatusDuration.DONT_CLEAR);
        setFocused(false);
        dispatch(setCustomStatus({...next, expires_at: next.duration === CustomStatusDuration.DATE_AND_TIME ? next.expires_at : expiresAt(next.duration || CustomStatusDuration.DONT_CLEAR)}));
        toast(formatMessage({id: 'fusion.toast.customStatus', defaultMessage: 'Custom status updated'}));
        setOfferReplies(away && autoReplies && status !== OUT_OF_OFFICE.status);
    };
    const turnOnReplies = () => {
        dispatch(updateMe({notify_props: {...me.notify_props, auto_responder_active: 'true', auto_responder_message: me.notify_props.auto_responder_message || formatMessage({id: 'fusion.status.autoReplyDefault', defaultMessage: 'Hello, I\'m out of office and unable to respond to messages.'})}}));
        toast(formatMessage({id: 'fusion.status.autoRepliesOn', defaultMessage: 'Automatic replies are on'}));
        onClose();
    };
    const pickClearAt = (at: number) => {
        const iso = new Date(at).toISOString();
        setDuration(CustomStatusDuration.DATE_AND_TIME);
        setClearAt(iso);
        setClearCustom(false);
        dispatch(setCustomStatus({emoji: emoji || 'speech_balloon', text: text.trim(), duration: CustomStatusDuration.DATE_AND_TIME, expires_at: iso}));
    };

    // The time a focus has left, in minutes, or hours and minutes.
    const focusLeft = (until: number) => {
        const minutes = Math.max(1, Math.ceil((until - Date.now()) / 6e4));
        if (minutes < 60) {
            return formatMessage({id: 'fusion.focus.leftMinutes', defaultMessage: '{minutes} min left', description: 'Status menu, while in focus mode: the minutes it has left.'}, {minutes});
        }
        return formatMessage({id: 'fusion.focus.leftHours', defaultMessage: '{hours} h {minutes} min left', description: 'Status menu, while in focus mode: the hours and minutes it has left.'}, {hours: Math.floor(minutes / 60), minutes: minutes % 60});
    };

    const choose = (s: string, endTime = 0) => {
        dispatch(setStatus({user_id: me.id, status: s, dnd_end_time: endTime}));
        toast(formatMessage({id: 'fusion.toast.status', defaultMessage: 'Status set to {status}'}, {status: label(s)}));
        onClose();
    };

    return (
        <Popover
            anchor={anchor}
            placement={anchor?.closest('.am-appbar, .am-pop') ? 'beside' : 'below'}
            className='status-pop'
            label={formatMessage({id: 'fusion.status.label', defaultMessage: 'Set your status'})}
            onClose={onClose}
        >
            <div className={am('sp-head')}>
                <Avatar
                    userId={me.id}
                    size='lg'
                    status={true}
                />
                <div>
                    <b>
                        {name}
                        <StatusEmoji userId={me.id}/>
                    </b>
                    <span>{focusLabel ? `@${me.username} · ${focusLabel}` : `@${me.username} · ${label(status)}${status === 'dnd' && dndEndTime > 0 ? ' · ' + formatMessage({id: 'fusion.status.until', defaultMessage: 'until {time}'}, {time: intl.formatDate(dndEndTime * 1000, {weekday: 'short', hour: 'numeric', minute: '2-digit'})}) : ''}`}</span>
                </div>
            </div>
            <form
                className={am('sp-custom')}
                onSubmit={saveCustom}
            >
                <button
                    ref={emojiButton}
                    type='button'
                    className={am('sp-emoji')}
                    title={formatMessage({id: 'fusion.status.emoji', defaultMessage: 'Choose an emoji'})}
                    aria-label={formatMessage({id: 'fusion.status.emoji', defaultMessage: 'Choose an emoji'})}
                    onClick={() => setPicking(!picking)}
                >
                    {emoji ? (
                        <RenderEmoji
                            emojiName={emoji}
                            size={18}
                        />
                    ) : (
                        <Icon
                            name='smile'
                            size='sm'
                        />
                    )}
                </button>
                <input
                    value={text}
                    placeholder={formatMessage({id: 'fusion.status.custom', defaultMessage: 'Set a custom status'})}
                    aria-label={formatMessage({id: 'fusion.status.customLabel', defaultMessage: 'Custom status'})}
                    maxLength={100}
                    onChange={(e) => {
                        setText(e.target.value);
                        setTyped(true);
                    }}
                    onFocus={() => {
                        setFocused(true);
                        setTyped(false);
                    }}
                    onBlur={() => {
                        setFocused(false);
                        saveCustom();
                    }}
                />
                {(text.trim() || emoji) && (
                    <button
                        ref={clockButton}
                        type='button'
                        className={am('sp-clock', {on: choosingDuration})}
                        title={formatMessage({id: 'fusion.status.clearAfterValue', defaultMessage: 'Clear after: {duration}'}, {duration: durationLabel})}
                        aria-label={formatMessage({id: 'fusion.status.clearAfterValue', defaultMessage: 'Clear after: {duration}'}, {duration: durationLabel})}
                        aria-haspopup='menu'
                        aria-expanded={choosingDuration}
                        onClick={() => setChoosingDuration(!choosingDuration)}
                    >
                        <Icon
                            name='clock'
                            size='sm'
                        />
                    </button>
                )}
                {(custom?.text || custom?.emoji) && (
                    <button
                        type='button'
                        aria-label={formatMessage({id: 'fusion.status.clear', defaultMessage: 'Clear custom status'})}
                        onClick={() => {
                            setText('');
                            setEmoji('');
                            dispatch(unsetCustomStatus());
                            toast(formatMessage({id: 'fusion.toast.customStatusCleared', defaultMessage: 'Custom status cleared'}));
                        }}
                    >
                        <Icon
                            name='x'
                            size='xs'
                        />
                    </button>
                )}
            </form>
            {choosingDuration && (
                <Popover
                    anchor={clockButton.current}
                    placement='below'
                    className='plus-pop sp-durations'
                    role='menu'
                    label={formatMessage({id: 'fusion.status.clearAfter', defaultMessage: 'Clear after'})}
                    onClose={() => setChoosingDuration(false)}
                >
                    <MenuHeading>{formatMessage({id: 'fusion.status.clearAfter', defaultMessage: 'Clear after'})}</MenuHeading>
                    {CLEAR_AFTER.map((d) => (
                        <MenuItem
                            key={d || 'never'}
                            role='menuitemradio'
                            checked={duration === d}
                            label={clearAfterLabel(formatMessage, d)}
                            onClick={() => {
                                setDuration(d);
                                setChoosingDuration(false);
                                dispatch(setCustomStatus({emoji: emoji || 'speech_balloon', text: text.trim(), duration: d, expires_at: expiresAt(d)}));
                            }}
                        />
                    ))}
                    <MenuItem
                        role='menuitemradio'
                        checked={duration === CustomStatusDuration.DATE_AND_TIME}
                        label={formatMessage({id: 'fusion.status.clearCustom', defaultMessage: 'Choose date and time…'})}
                        onClick={() => {
                            setChoosingDuration(false);
                            setClearCustom(true);
                        }}
                    />
                </Popover>
            )}
            {clearCustom && (
                <div className={am('sp-dnd-custom')}>
                    <CustomTimeForm
                        label={formatMessage({id: 'fusion.status.clearAt', defaultMessage: 'Clear the status on'})}
                        submitLabel={formatMessage({id: 'fusion.status.clearAtSet', defaultMessage: 'Set'})}
                        onPick={pickClearAt}
                    />
                </div>
            )}
            {focused && (!typed || !text.trim()) && (
                <div
                    className={am('sp-suggest')}
                    role='listbox'
                    aria-label={formatMessage({id: 'fusion.status.suggestions', defaultMessage: 'Status suggestions'})}
                    onMouseDown={(e) => e.preventDefault()}
                >
                    {recents.length > 0 && <MenuHeading>{formatMessage({id: 'fusion.status.recent', defaultMessage: 'Recent'})}</MenuHeading>}
                    {recents.slice(0, 5).map((recent) => (
                        <div
                            key={`${recent.emoji}:${recent.text}:${recent.duration}`}
                            className={am('sp-suggestion')}
                        >
                            <button
                                type='button'
                                role='option'
                                aria-selected={false}
                                onClick={() => applyStatus(recent)}
                            >
                                <RenderEmoji
                                    emojiName={recent.emoji}
                                    size={18}
                                />
                                <span className={am('grow')}>{recent.text}</span>
                            </button>
                            <button
                                type='button'
                                className={am('sp-forget')}
                                aria-label={formatMessage({id: 'fusion.status.forget', defaultMessage: 'Remove from recents'})}
                                onClick={() => dispatch(removeRecentCustomStatus(recent))}
                            >
                                <Icon
                                    name='x'
                                    size='xs'
                                />
                            </button>
                        </div>
                    ))}
                    <MenuHeading>{formatMessage({id: 'fusion.status.suggested', defaultMessage: 'Suggestions'})}</MenuHeading>
                    {SUGGESTIONS.filter((suggestion) => !recents.some((recent) => recent.emoji === suggestion.emoji && recent.text === formatMessage(suggestion.text))).map((suggestion) => (
                        <div
                            key={suggestion.text.id}
                            className={am('sp-suggestion')}
                        >
                            <button
                                type='button'
                                role='option'
                                aria-selected={false}
                                onClick={() => applyStatus({emoji: suggestion.emoji, text: formatMessage(suggestion.text), duration: suggestion.duration}, suggestion.away)}
                            >
                                <RenderEmoji
                                    emojiName={suggestion.emoji}
                                    size={18}
                                />
                                <span className={am('grow')}>{formatMessage(suggestion.text)}</span>
                                <small>{clearAfterLabel(formatMessage, suggestion.duration)}</small>
                            </button>
                        </div>
                    ))}
                </div>
            )}
            {offerReplies && (
                <div className={am('sp-offer')}>
                    <span>{formatMessage({id: 'fusion.status.offerReplies', defaultMessage: 'Also answer direct messages automatically while you\'re away?'})}</span>
                    <button
                        type='button'
                        className={am('btn')}
                        onClick={() => setOfferReplies(false)}
                    >
                        {formatMessage({id: 'fusion.status.offerNo', defaultMessage: 'No'})}
                    </button>
                    <button
                        type='button'
                        className={am('btn', 'primary')}
                        onClick={turnOnReplies}
                    >
                        {formatMessage({id: 'fusion.status.offerYes', defaultMessage: 'Turn on'})}
                    </button>
                </div>
            )}
            {picking && (
                <EmojiPicker
                    anchor={emojiButton.current}
                    onPick={(name) => {
                        setEmoji(name);
                        setPicking(false);
                        saveCustom(undefined, name);
                    }}
                    onClose={() => setPicking(false)}
                />
            )}
            {dndCustom && (
                <div className={am('sp-dnd-custom')}>
                    <CustomTimeForm
                        label={formatMessage({id: 'fusion.status.dndUntil', defaultMessage: 'Do not disturb until'})}
                        submitLabel={formatMessage({id: 'fusion.status.dndSet', defaultMessage: 'Turn on'})}
                        onPick={(at) => choose('dnd', Math.floor(at / 1000))}
                    />
                </div>
            )}
            <div className={am('sp-list')}>
                {status === OUT_OF_OFFICE.status && (
                    <button
                        className={am('sp-opt', 'on')}
                        aria-pressed={true}
                        title={formatMessage({id: 'fusion.status.oooHint', defaultMessage: 'Choose another status to turn the automatic replies off'})}
                        onClick={() => {
                            settings.open('notifications');
                            onClose();
                        }}
                    >
                        <span className={am('status-dot', 'ooo')}/>
                        <span className={am('grow')}>
                            <b>{formatMessage(OUT_OF_OFFICE.label)}</b>
                            <span>{formatMessage(OUT_OF_OFFICE.sub)}</span>
                        </span>
                        <Icon
                            name='check'
                            size='sm'
                        />
                    </button>
                )}
                {STATUSES.map((s) => {
                    // While focusing, the focus row is the one on, not Do not disturb.
                    const on = status === s.status && !(focusUntil && s.status === 'dnd');
                    const option = (
                        <button
                            className={am('sp-opt', {on})}
                            aria-pressed={on}
                            aria-haspopup={s.status === 'dnd' ? 'menu' : undefined}
                            onClick={() => choose(s.status)}
                        >
                            <span className={am('status-dot', s.status)}/>
                            <span className={am('grow')}>
                                <b>{formatMessage(s.label)}</b>
                                {s.sub && <span>{formatMessage(s.sub)}</span>}
                            </span>
                            {on && (
                                <Icon
                                    name='check'
                                    size='sm'
                                />
                            )}
                            {s.status === 'dnd' && (
                                <span className={am('chev')}>
                                    <Icon
                                        name='chev'
                                        size='xs'
                                    />
                                </span>
                            )}
                        </button>
                    );
                    // Focus mode sits between Do not disturb and Invisible: it's a timed Do not disturb, so it
                    // belongs right after it.
                    const focusOption = s.status === 'dnd' && (
                        <button
                            key='focus'
                            className={am('sp-opt', 'sp-focus', {on: Boolean(focusUntil)})}
                            aria-pressed={Boolean(focusUntil)}
                            onClick={() => {
                                if (focusUntil) {
                                    Client4.endFocus().catch(() => toast(formatMessage({id: 'fusion.focus.endFailed', defaultMessage: 'Focus mode couldn\'t be ended', description: 'Toast when ending focus mode from the status menu failed.'})));
                                } else {
                                    dialogs.focus();
                                }
                                onClose();
                            }}
                        >
                            <span className={am('status-dot', 'focus')}/>
                            <span className={am('grow')}>
                                <b>{focusUntil ? formatMessage({id: 'fusion.focus.end', defaultMessage: 'End focus', description: 'Status menu (your avatar), while in focus mode: ends it now.'}) : formatMessage({id: 'fusion.focus.menu', defaultMessage: 'Focus mode…', description: 'Status menu (your avatar): opens the dialog to start focus mode (Do not disturb for a while, with a reason).'})}</b>
                                <span>{focusUntil ? `${focusLeft(focusUntil)} · ${formatMessage({id: 'fusion.focus.untilShort', defaultMessage: 'until {time}', description: 'Status menu, while in focus mode: when it ends.'}, {time: focusTime})}` : formatMessage({id: 'fusion.focus.menuSub', defaultMessage: 'Mutes notifications and hides your unread badges for a while', description: 'Status menu (your avatar): under "Focus mode…", what it does.'})}</span>
                            </span>
                            {focusUntil > 0 && (
                                <Icon
                                    name='check'
                                    size='sm'
                                />
                            )}
                        </button>
                    );
                    if (s.status !== 'dnd') {
                        return <React.Fragment key={s.status}>{option}</React.Fragment>;
                    }
                    return (
                        <React.Fragment key={s.status}>
                            <div className={am('sp-item')}>
                                {option}
                                <div
                                    className={am('sp-sub')}
                                    role='menu'
                                    aria-label={formatMessage({id: 'fusion.status.dndFor', defaultMessage: 'Do not disturb for'})}
                                >
                                    {DND_DURATIONS.map(([key, ms]) => (
                                        <button
                                            key={key}
                                            role='menuitem'
                                            onClick={() => choose('dnd', dndEnd(ms))}
                                        >
                                            {dndDurationLabel(formatMessage, key)}
                                        </button>
                                    ))}
                                    <button
                                        role='menuitem'
                                        onClick={() => setDndCustom(true)}
                                    >
                                        {formatMessage({id: 'fusion.status.dndCustom', defaultMessage: 'Until a custom time…'})}
                                    </button>
                                </div>
                            </div>
                            {focusOption}
                        </React.Fragment>
                    );
                })}
            </div>
            <div className={am('sp-links')}>
                <button
                    onClick={() => {
                        settings.open('account');
                        onClose();
                    }}
                >
                    <Icon
                        name='cog'
                        size='sm'
                    />
                    {formatMessage({id: 'fusion.status.settings', defaultMessage: 'Profile & settings'})}
                </button>
                {/* A community's integrations are in its settings (its menu › Community settings › Integrations). */}
                {systemConsole && (
                    <button onClick={() => getHistory().push('/admin_console')}>
                        <Icon
                            name='shield'
                            size='sm'
                        />
                        {formatMessage({id: 'fusion.status.console', defaultMessage: 'System Console'})}
                    </button>
                )}
                {marketplace && (
                    <button
                        onClick={() => {
                            dispatch(openModal({modalId: ModalIdentifiers.PLUGIN_MARKETPLACE, dialogType: MarketplaceModal}));
                            onClose();
                        }}
                    >
                        <Icon
                            name='store'
                            size='sm'
                        />
                        {formatMessage({id: 'fusion.status.marketplace', defaultMessage: 'App Marketplace', description: 'Status menu (your avatar): opens the App Marketplace, to install apps; shown to those who may install apps.'})}
                    </button>
                )}
                <button
                    className={am('danger')}
                    onClick={() => emitUserLoggedOutEvent()}
                >
                    <Icon
                        name='export'
                        size='sm'
                    />
                    {formatMessage({id: 'fusion.status.logout', defaultMessage: 'Log out'})}
                </button>
            </div>
        </Popover>
    );
}
