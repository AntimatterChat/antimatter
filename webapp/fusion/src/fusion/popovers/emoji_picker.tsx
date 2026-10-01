// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useSelector} from 'react-redux';

import type {Emoji} from '@mattermost/types/emojis';

import {isSystemEmoji} from 'mattermost-redux/utils/emoji_utils';

import {getEmojiMap, getRecentEmojisNames, getUserSkinTone} from 'selectors/emojis';

import RenderEmoji from 'components/emoji/render_emoji';
import {getFilteredEmojis, getUpdatedCategoriesAndAllEmojis} from 'components/emoji_picker/utils';

import Icon from 'fusion/components/icon';
import {Popover} from 'fusion/components/layer';
import {am} from 'fusion/utils/class_names';

const nameOf = (emoji: Emoji) => (isSystemEmoji(emoji) ? emoji.short_names[0] : emoji.name);
const idOf = (emoji: Emoji) => (isSystemEmoji(emoji) ? emoji.unified.toLowerCase() : emoji.id);

const ROW_HEIGHT = 34;
const PER_ROW = 8;

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

type Props = {
    anchor: HTMLElement | null;
    onPick: (emojiName: string) => void;
    onClose: () => void;

    // Reacting closes the picker; the composer keeps it open to insert several emoji.
    keepOpen?: boolean;
};

// EmojiPicker is the mockup's picker: search, category rail, grid and a footer naming the emoji under the pointer.
export default function EmojiPicker({anchor, onPick, onClose, keepOpen = false}: Props) {
    const {formatMessage} = useIntl();
    const emojiMap = useSelector(getEmojiMap);
    const recent = useSelector(getRecentEmojisNames);
    const skinTone = useSelector(getUserSkinTone);
    const [query, setQuery] = useState('');
    const [hovered, setHovered] = useState<Emoji | null>(null);
    const scrollRef = useRef<HTMLDivElement>(null);
    const inputRef = useRef<HTMLInputElement>(null);

    const [categories, allEmojis] = useMemo(() => getUpdatedCategoriesAndAllEmojis(emojiMap, recent, skinTone, {}), [emojiMap, recent, skinTone]);
    const results = useMemo(() => (query.trim() ? getFilteredEmojis(allEmojis, query.trim(), recent, skinTone) : null), [allEmojis, query, recent, skinTone]);

    useEffect(() => {
        inputRef.current?.focus();
    }, []);

    const pick = (emoji: Emoji) => {
        onPick(nameOf(emoji));
        if (!keepOpen) {
            onClose();
        }
    };
    const sections = Object.values(categories).filter((c) => c.emojiIds?.length);

    return (
        <Popover
            anchor={anchor}
            placement='above'
            className='picker'
            label={formatMessage({id: 'fusion.picker.label', defaultMessage: 'Emoji'})}
            onClose={onClose}
        >
            <div className={am('picker-search')}>
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
                        onChange={(e) => setQuery(e.target.value)}
                        onKeyDown={(e) => {
                            if (e.key === 'Enter' && results?.length) {
                                e.preventDefault();
                                pick(results[0]);
                            }
                        }}
                    />
                </label>
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
        </Popover>
    );
}
