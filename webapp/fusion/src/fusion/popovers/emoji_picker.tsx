// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {shallowEqual, useDispatch, useSelector} from 'react-redux';

import type {Emoji} from '@mattermost/types/emojis';

import Permissions from 'mattermost-redux/constants/permissions';
import {getCustomEmojisEnabled} from 'mattermost-redux/selectors/entities/emojis';
import {getCurrentTeam} from 'mattermost-redux/selectors/entities/teams';
import {isSystemEmoji} from 'mattermost-redux/utils/emoji_utils';

import {addRecentEmoji} from 'actions/emoji_actions';
import {getEmojiPickerTabs} from 'selectors/emoji_picker_tabs';
import {getEmojiMap, getRecentEmojisNames, getUserSkinTone} from 'selectors/emojis';

import RenderEmoji from 'components/emoji/render_emoji';
import {getFilteredEmojis, getUpdatedCategoriesAndAllEmojis} from 'components/emoji_picker/utils';
import AnyTeamPermissionGate from 'components/permissions_gates/any_team_permission_gate';

import Icon, {isIconName} from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';
import {openCustomEmoji} from 'fusion/utils/modals';
import PluggableErrorBoundary from 'plugins/pluggable/error_boundary';

import type {GlobalState} from 'types/store';
import type {EmojiPickerTabRegistration} from 'types/store/plugins';

import SkinTonePicker from './skin_tone_picker';

const nameOf = (emoji: Emoji) => (isSystemEmoji(emoji) ? emoji.short_names[0] : emoji.name);
const idOf = (emoji: Emoji) => (isSystemEmoji(emoji) ? emoji.unified.toLowerCase() : emoji.id);

const ROW_HEIGHT = 34;
const PER_ROW = 8;

const DEFAULT_RECENT = ['+1', 'heart', 'joy', 'tada', 'eyes', 'white_check_mark', 'pray', 'fire'];

// A category's emoji grid, drawn once it scrolls into view, since there are nearly two thousand emoji.
function LazyGrid({emojis, onPick, onHover, root}: {emojis: Emoji[]; onPick: (e: Emoji) => void; onHover: (e: Emoji) => void; root: HTMLElement | null}) {
    const ref = useRef<HTMLDivElement>(null);
    const [visible, setVisible] = useState(false);
    useEffect(() => {
        if (!ref.current || visible) {
            return undefined;
        }
        const observer = new IntersectionObserver((entries) => {
            if (entries.some((e) => e.isIntersecting)) {
                setVisible(true);
            }
        }, {root, rootMargin: '200px'});
        observer.observe(ref.current);
        return () => observer.disconnect();
    }, [root, visible]);
    return (
        <div
            ref={ref}
            className={am('emoji-grid')}
            style={visible ? undefined : {height: Math.ceil(emojis.length / PER_ROW) * (ROW_HEIGHT + 2)}}
        >
            {visible && emojis.map((emoji) => (
                <button
                    key={idOf(emoji)}
                    type='button'
                    title={`:${nameOf(emoji)}:`}
                    aria-label={nameOf(emoji)}
                    onClick={() => onPick(emoji)}
                    onMouseEnter={() => onHover(emoji)}
                >
                    <RenderEmoji
                        emojiName={nameOf(emoji)}
                        size={22}
                    />
                </button>
            ))}
        </div>
    );
}

// The message box a composer's picker belongs to: its picker also shows the plugins' tabs (GIFs, stickers...).
export type PickerCompose = {
    channelId: string;
    rootId?: string;
    insertText: (text: string) => void;
};

type Props = {
    anchor: HTMLElement | null;
    onPick: (emojiName: string) => void;
    onClose: () => void;

    // Reacting closes the picker; the composer keeps it open to insert several emoji.
    keepOpen?: boolean;
    compose?: PickerCompose;
};

const EMOJI_TAB = 'emoji';

// The order of the Emoji tab among the plugins' tabs: theirs before 10 come first (GIFs, Stickers), the others after
// (Memes), as in the mockup's picker.
export const EMOJI_TAB_ORDER = 10;
const NO_TABS: EmojiPickerTabRegistration[] = [];

// The composer's picker opens on the tab used last, as in the mockup.
let lastTab = EMOJI_TAB;
const rememberTab = (id: string) => {
    lastTab = id;
};

type EmojiTabProps = {
    query: string;
    onQuery: (query: string) => void;
    onPick: (emoji: Emoji) => void;
    onClose: () => void;
    keepOpen: boolean;
};

// EmojiTab is the mockup's emoji picker: search, category rail, grid and a footer naming the emoji under the pointer.
function EmojiTab({query, onQuery, onPick: pick, onClose, keepOpen}: EmojiTabProps) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const customEmojisEnabled = useSelector(getCustomEmojisEnabled);
    const teamName = useSelector((state: GlobalState) => getCurrentTeam(state)?.name ?? '');
    const emojiMap = useSelector(getEmojiMap);
    const used = useSelector(getRecentEmojisNames);

    // The "Recently used" line comes first; until anything was used, it holds the usual reactions.
    const recent = used.length ? used : DEFAULT_RECENT;
    const skinTone = useSelector(getUserSkinTone);
    const [hovered, setHovered] = useState<Emoji | null>(null);
    const scrollRef = useRef<HTMLDivElement>(null);
    const inputRef = useRef<HTMLInputElement>(null);

    const [categories, allEmojis] = useMemo(() => getUpdatedCategoriesAndAllEmojis(emojiMap, recent, skinTone, {}), [emojiMap, recent, skinTone]);

    // The search ranks the recently used emoji first by their ids (as allEmojis has them), not by their names.
    const recentIds = categories.recent?.emojiIds;
    const results = useMemo(() => (query.trim() ? getFilteredEmojis(allEmojis, query.trim(), recentIds ?? [], skinTone) : null), [allEmojis, query, recentIds, skinTone]);

    useEffect(() => {
        inputRef.current?.focus();
    }, []);

    const sections = Object.values(categories).filter((c) => c.emojiIds?.length);

    return (
        <>
            <div className={am('picker-search', 'picker-search-row')}>
                <label>
                    <Icon
                        name='search'
                        size='sm'
                    />
                    <input
                        ref={inputRef}
                        value={query}
                        placeholder={formatMessage({id: 'fusion.picker.search', defaultMessage: 'Search emoji'})}
                        aria-label={formatMessage({id: 'fusion.picker.search', defaultMessage: 'Search emoji'})}
                        autoComplete='off'
                        onChange={(e) => onQuery(e.target.value)}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter' && results?.length) {
                                e.preventDefault();
                                pick(results[0]);
                            }
                        }}
                    />
                </label>
                <SkinTonePicker/>
                {customEmojisEnabled && teamName && (
                    <AnyTeamPermissionGate permissions={[Permissions.CREATE_EMOJIS]}>
                        <button
                            type='button'
                            className={am('icon-btn')}
                            title={formatMessage({id: 'fusion.picker.addCustomEmoji', defaultMessage: 'Add custom emoji'})}
                            aria-label={formatMessage({id: 'fusion.picker.addCustomEmoji', defaultMessage: 'Add custom emoji'})}
                            onClick={() => {
                                onClose();
                                dispatch(openCustomEmoji());
                            }}
                        >
                            <Icon name='plus'/>
                        </button>
                    </AnyTeamPermissionGate>
                )}
            </div>
            <div className={am('picker-body')}>
                {!results && (
                    <nav
                        className={am('emoji-nav')}
                        aria-label={formatMessage({id: 'fusion.picker.categories', defaultMessage: 'Emoji categories'})}
                    >
                        {sections.map((c) => {
                            const first = allEmojis[(c.emojiIds || [])[0]];
                            return (
                                <button
                                    key={c.name}
                                    type='button'
                                    title={formatMessage(c.label)}
                                    aria-label={formatMessage(c.label)}
                                    onClick={() => document.getElementById(`am-ecat-${c.name}`)?.scrollIntoView()}
                                >
                                    {first && (
                                        <RenderEmoji
                                            emojiName={nameOf(first)}
                                            size={18}
                                        />
                                    )}
                                </button>
                            );
                        })}
                    </nav>
                )}
                <div
                    ref={scrollRef}
                    className={am('emoji-scroll')}
                >
                    {results ? (
                        <>
                            <h5>{formatMessage({id: 'fusion.picker.results', defaultMessage: 'Search results'})}</h5>
                            {results.length ? (
                                <LazyGrid
                                    emojis={results.slice(0, 200)}
                                    onPick={pick}
                                    onHover={setHovered}
                                    root={scrollRef.current}
                                />
                            ) : (
                                <div className={am('empty')}>{formatMessage({id: 'fusion.picker.none', defaultMessage: 'No emoji match.'})}</div>
                            )}
                        </>
                    ) : sections.map((c) => (
                        <React.Fragment key={c.name}>
                            <h5 id={`am-ecat-${c.name}`}>{formatMessage(c.label)}</h5>
                            <LazyGrid
                                emojis={(c.emojiIds || []).map((id) => allEmojis[id]).filter(Boolean)}
                                onPick={pick}
                                onHover={setHovered}
                                root={scrollRef.current}
                            />
                        </React.Fragment>
                    ))}
                </div>
            </div>
            <div className={am('picker-foot')}>
                <span className={am('big')}>
                    <RenderEmoji
                        emojiName={hovered ? nameOf(hovered) : '+1'}
                        size={26}
                    />
                </span>
                <span>{keepOpen ? formatMessage({id: 'fusion.picker.insert', defaultMessage: 'Insert'}) : formatMessage({id: 'fusion.picker.reactWith', defaultMessage: 'React with'})}</span>
                <code>{`:${hovered ? nameOf(hovered) : '+1'}:`}</code>
            </div>
        </>
    );
}

// EmojiPicker is the mockup's picker. The composer's has tabs when plugins add some (registerEmojiPickerTab), in
// the mockup's order: theirs (GIFs, Stickers), Emoji, then theirs of order 10 or more (Memes). A plugin tab renders
// its own search box and body.
export default function EmojiPicker({anchor, onPick, onClose, keepOpen = false, compose}: Props) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const channelId = compose?.channelId || '';
    const rootId = compose?.rootId || undefined;
    const ctx = useMemo(() => ({channelId, rootId}), [channelId, rootId]);
    const pluginTabs = useSelector((state: GlobalState) => (compose ? getEmojiPickerTabs(state, ctx) : NO_TABS), shallowEqual);
    const [selected, setSelected] = useState(compose ? lastTab : EMOJI_TAB);
    const [query, setQuery] = useState('');

    // A tab that went away (e.g. its plugin was disabled) leaves the Emoji tab selected.
    const pluginTab = pluginTabs.find((t) => t.id === selected);

    const selectTab = (id: string) => {
        setSelected(id);
        rememberTab(id);
        setQuery('');
    };

    const pick = (emoji: Emoji) => {
        // Picked emoji lead the "Recently used" line.
        dispatch(addRecentEmoji(nameOf(emoji)));
        onPick(nameOf(emoji));
        if (!keepOpen) {
            onClose();
        }
    };

    const asTab = (t: typeof pluginTabs[number]) => ({id: t.id, label: t.label, icon: t.fusionIcon && isIconName(t.fusionIcon) ? t.fusionIcon : undefined});
    const tabs = [
        ...pluginTabs.filter((t) => t.order < EMOJI_TAB_ORDER).map(asTab),
        {id: EMOJI_TAB, label: formatMessage({id: 'fusion.picker.tabEmoji', defaultMessage: 'Emoji'}), icon: undefined},
        ...pluginTabs.filter((t) => t.order >= EMOJI_TAB_ORDER).map(asTab),
    ];
    const current = pluginTab ? pluginTab.id : EMOJI_TAB;

    return (
        <Popover
            anchor={anchor}
            placement='picker'
            className='picker'
            label={formatMessage({id: 'fusion.picker.label', defaultMessage: 'Emoji'})}
            onClose={onClose}
        >
            {pluginTabs.length > 0 && (
                <div
                    className={am('picker-tabs')}
                    role='tablist'
                >
                    {tabs.map((t) => (
                        <button
                            key={t.id}
                            type='button'
                            role='tab'
                            aria-selected={current === t.id}
                            className={am({on: current === t.id})}
                            onClick={() => selectTab(t.id)}
                        >
                            {t.icon && (
                                <Icon
                                    name={t.icon}
                                    size='sm'
                                />
                            )}
                            {t.label}
                        </button>
                    ))}
                </div>
            )}
            {pluginTab ? (
                <PluggableErrorBoundary
                    key={pluginTab.id}
                    pluginId={pluginTab.pluginId}
                >
                    <pluginTab.component
                        channelId={channelId}
                        rootId={rootId}
                        filter={query}
                        onFilterChange={setQuery}
                        onSelectDone={onClose}
                        isFusion={true}
                        insertText={compose?.insertText}
                    />
                </PluggableErrorBoundary>
            ) : (
                <EmojiTab
                    query={query}
                    onQuery={setQuery}
                    onPick={pick}
                    onClose={onClose}
                    keepOpen={keepOpen}
                />
            )}
        </Popover>
    );
}
