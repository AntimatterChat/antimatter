// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useCallback, useMemo, useRef, useState} from 'react';
import {Tab, Tabs} from 'react-bootstrap';
import {FormattedMessage, useIntl} from 'react-intl';
import {shallowEqual, useSelector} from 'react-redux';

import type {Emoji} from '@mattermost/types/emojis';

import {getEmojiPickerTabs, isGifPickerReplaced} from 'selectors/emoji_picker_tabs';

import {makeAsyncComponent} from 'components/async_load';
import EmojiPicker from 'components/emoji_picker';
import EmojiPickerHeader from 'components/emoji_picker/components/emoji_picker_header';
import EmojiIcon from 'components/widgets/icons/emoji_icon';
import GifIcon from 'components/widgets/icons/giphy_icon';

import PluggableErrorBoundary from 'plugins/pluggable/error_boundary';

import type {GlobalState} from 'types/store';
import type {EmojiPickerTabRegistration} from 'types/store/plugins';

const GifPicker = makeAsyncComponent('GifPicker', React.lazy(() => import('components/gif_picker/gif_picker')));

export interface Props {
    onEmojiClose: () => void;
    onEmojiClick: (emoji: Emoji) => void;
    onGifClick?: (gif: string) => void;
    onAddCustomEmojiClick?: () => void;
    enableGifPicker?: boolean;

    /** The channel of the message box, and its thread for a reply box. */
    channelId?: string;
    rootId?: string;

    /** Whether to show the tabs of plugins (registerEmojiPickerTab): when composing a message. */
    showPluginTabs?: boolean;

    /** Inserts text at the caret of the message box, for the plugin tabs. */
    insertText?: (text: string) => void;
}

const NO_PLUGIN_TABS: EmojiPickerTabRegistration[] = [];

const EMOJI_TAB = 1;
const GIF_TAB = 2;
const pluginTabKey = (tab: EmojiPickerTabRegistration) => `plugin_${tab.id}`;

export default function EmojiPickerTabs(props: Props) {
    const intl = useIntl();

    const [activeKey, setActiveKey] = useState<number | string>(EMOJI_TAB);
    const [filter, setFilter] = useState('');

    const rootPickerNodeRef = useRef<HTMLDivElement>(null);
    const getRootPickerNode = useCallback(() => rootPickerNodeRef.current, []);

    const ctx = useMemo(() => ({channelId: props.channelId || '', rootId: props.rootId || undefined}), [props.channelId, props.rootId]);
    const canShowGifPicker = Boolean(props.enableGifPicker && typeof props.onGifClick != 'undefined');
    const gifPickerReplaced = useSelector((state: GlobalState) => canShowGifPicker && isGifPickerReplaced(state, ctx));
    const pluginTabs = useSelector((state: GlobalState) => (props.showPluginTabs ? getEmojiPickerTabs(state, ctx) : NO_PLUGIN_TABS), shallowEqual);
    const showGifPicker = canShowGifPicker && !gifPickerReplaced;

    const {onEmojiClose} = props;
    const pluginTabProps = {
        channelId: ctx.channelId || undefined,
        rootId: ctx.rootId,
        filter,
        onFilterChange: setFilter,
        onSelectDone: onEmojiClose,
        isFusion: false,
        insertText: props.insertText,
    };

    // A tab that went away (e.g. its plugin was disabled) leaves the Emojis tab selected.
    const tabKeys: Array<number | string> = [EMOJI_TAB, ...(showGifPicker ? [GIF_TAB] : []), ...pluginTabs.map(pluginTabKey)];
    const currentKey = tabKeys.includes(activeKey) ? activeKey : EMOJI_TAB;

    let dialogLabel = intl.formatMessage({id: 'emoji_gif_picker.dialog.emojis', defaultMessage: 'Emoji Picker'});
    if (currentKey === GIF_TAB) {
        dialogLabel = intl.formatMessage({id: 'emoji_gif_picker.dialog.gifs', defaultMessage: 'GIF Picker'});
    } else if (currentKey !== EMOJI_TAB) {
        const label = pluginTabs.find((tab) => pluginTabKey(tab) === currentKey)?.label;
        if (typeof label === 'string') {
            dialogLabel = label;
        }
    }

    if (showGifPicker || pluginTabs.length > 0) {
        return (
            <div
                id='emojiGifPicker'
                ref={rootPickerNodeRef}
                className='a11y__popup emoji-picker'
                role='dialog'
                aria-label={dialogLabel}
                aria-modal='true'
            >
                <EmojiPickerHeader handleEmojiPickerClose={props.onEmojiClose}/>
                <Tabs
                    id='emoji-picker-tabs'
                    defaultActiveKey={EMOJI_TAB}
                    justified={true}
                    mountOnEnter={true}
                    unmountOnExit={true}
                    activeKey={currentKey}
                    onSelect={(activeKey) => setActiveKey(activeKey)}
                >
                    <Tab
                        eventKey={EMOJI_TAB}
                        title={
                            <div className={'custom-emoji-tab__icon__text'}>
                                <EmojiIcon
                                    className='custom-emoji-tab__icon'
                                    aria-hidden={true}
                                />
                                <FormattedMessage
                                    id='emoji_gif_picker.tabs.emojis'
                                    defaultMessage='Emojis'
                                />
                            </div>
                        }
                        unmountOnExit={true}
                        tabClassName={'custom-emoji-tab'}
                    >
                        <EmojiPicker
                            filter={filter}
                            onEmojiClick={props.onEmojiClick}
                            handleFilterChange={setFilter}
                            handleEmojiPickerClose={props.onEmojiClose}
                        />
                    </Tab>
                    {showGifPicker && (
                        <Tab
                            eventKey={GIF_TAB}
                            title={
                                <div className={'custom-emoji-tab__icon__text'}>
                                    <GifIcon
                                        className='custom-emoji-tab__icon'
                                        aria-hidden={true}
                                    />
                                    <FormattedMessage
                                        id='emoji_gif_picker.tabs.gifs'
                                        defaultMessage='GIFs'
                                    />
                                </div>
                            }
                            unmountOnExit={true}
                            tabClassName={'custom-emoji-tab'}
                        >
                            <GifPicker
                                filter={filter}
                                getRootPickerNode={getRootPickerNode}
                                onGifClick={props.onGifClick}
                                handleFilterChange={setFilter}
                            />
                        </Tab>
                    )}
                    {pluginTabs.map((tab) => (
                        <Tab
                            key={tab.id}
                            eventKey={pluginTabKey(tab)}
                            title={
                                <div className={'custom-emoji-tab__icon__text'}>
                                    {tab.icon && (
                                        <span
                                            className='custom-emoji-tab__icon'
                                            aria-hidden={true}
                                        >
                                            {tab.icon}
                                        </span>
                                    )}
                                    {tab.label}
                                </div>
                            }
                            unmountOnExit={true}
                            tabClassName={'custom-emoji-tab'}
                        >
                            <PluggableErrorBoundary pluginId={tab.pluginId}>
                                <tab.component {...pluginTabProps}/>
                            </PluggableErrorBoundary>
                        </Tab>
                    ))}
                </Tabs>
            </div>
        );
    }

    return (
        <div
            id='emojiPicker'
            className='a11y__popup emoji-picker emoji-picker--single'
            role='dialog'
            aria-label={intl.formatMessage({id: 'emoji_gif_picker.dialog.emojis', defaultMessage: 'Emoji Picker'})}
            aria-modal='true'
        >
            <EmojiPickerHeader handleEmojiPickerClose={props.onEmojiClose}/>
            <EmojiPicker
                filter={filter}
                onEmojiClick={props.onEmojiClick}
                handleFilterChange={setFilter}
                handleEmojiPickerClose={props.onEmojiClose}
                onAddCustomEmojiClick={props.onAddCustomEmojiClick}
            />
        </div>
    );
}
