// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useSyncExternalStore} from 'react';
import {useIntl} from 'react-intl';

import type {Post} from '@mattermost/types/posts';

import Icon from 'fusion/components/icon';
import {am} from 'fusion/utils/class_names';
import {getPostTranslation} from 'utils/post_utils';

// The messages shown in their original language, for the session (as spoilers shown).
const originals = new Set<string>();
const listeners = new Set<() => void>();
const subscribe = (cb: () => void) => {
    listeners.add(cb);
    return () => listeners.delete(cb);
};

export function useShowOriginal(postId: string): [boolean, (show: boolean) => void] {
    const shown = useSyncExternalStore(subscribe, () => originals.has(postId));
    const set = (show: boolean) => {
        if (show) {
            originals.add(postId);
        } else {
            originals.delete(postId);
        }
        listeners.forEach((cb) => cb());
    };
    return [shown, set];
}

// languageName names a language code in the reader's language ("fr" → "French").
function languageName(code: string, locale: string): string {
    try {
        return new Intl.DisplayNames([locale], {type: 'language'}).of(code) || code;
    } catch {
        return code;
    }
}

// TranslationNote says, under a message of an auto-translated channel, that it was translated and from what
// language, with a switch to its original text; or that it's being translated.
export default function TranslationNote({post, showOriginal, onToggle}: {post: Post; showOriginal: boolean; onToggle: () => void}) {
    const {formatMessage, locale} = useIntl();
    const translation = getPostTranslation(post, locale);
    if (post.type !== '' || !translation) {
        return null;
    }
    if (translation.state === 'processing') {
        return (
            <div className={am('translation-note', 'busy')}>
                <Icon
                    name='globe'
                    size='xs'
                />
                {formatMessage({id: 'fusion.translation.processing', defaultMessage: 'Translating…'})}
            </div>
        );
    }
    if (translation.state !== 'ready') {
        return null;
    }
    const from = translation.source_lang ? languageName(translation.source_lang, locale) : '';
    return (
        <div className={am('translation-note')}>
            <Icon
                name='globe'
                size='xs'
            />
            <span>
                {showOriginal ? formatMessage({id: 'fusion.translation.original', defaultMessage: 'Original message'}) : formatMessage({id: 'fusion.translation.translated', defaultMessage: 'Translated{from, select, none {} other { from {from}}}'}, {from: from || 'none'})}
            </span>
            <button
                type='button'
                onClick={onToggle}
            >
                {showOriginal ? formatMessage({id: 'fusion.translation.showTranslation', defaultMessage: 'Show the translation'}) : formatMessage({id: 'fusion.translation.showOriginal', defaultMessage: 'Show the original'})}
            </button>
        </div>
    );
}
