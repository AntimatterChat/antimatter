// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useLayoutEffect, useRef, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {deletePreferences, savePreferences} from 'mattermost-redux/actions/preferences';
import {get as getPreference} from 'mattermost-redux/selectors/entities/preferences';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {am} from 'fusion/utils/class_names';

import type {GlobalState} from 'types/store';

// A private note on someone, as Discord's: only its writer sees it, kept in their preferences.
export const CATEGORY_USER_NOTE = 'fusion_user_note';
export const USER_NOTE_MAX = 500;

// UserNote is the note's field in a profile card, saved when it loses the focus, and when the card closes (by Escape
// or a click elsewhere, which take the card away before the field hears of it).
export default function UserNote({userId}: {userId: string}) {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const me = useSelector(getCurrentUserId);
    const saved = useSelector((state: GlobalState) => getPreference(state, CATEGORY_USER_NOTE, userId, ''));
    const [text, setText] = useState(saved);
    useEffect(() => setText(saved), [saved]);

    // What save reads, current even from the unmount cleanup.
    const latest = useRef({text, saved});
    latest.current = {text, saved};

    const save = () => {
        const value = latest.current.text.trim();
        const was = latest.current.saved;
        if (value === was) {
            return;
        }
        latest.current.saved = value;
        if (value) {
            dispatch(savePreferences(me, [{user_id: me, category: CATEGORY_USER_NOTE, name: userId, value}]));
        } else {
            dispatch(deletePreferences(me, [{user_id: me, category: CATEGORY_USER_NOTE, name: userId, value: was}]));
        }
    };
    useEffect(() => save, []); // eslint-disable-line react-hooks/exhaustive-deps

    // The field is as tall as its text, from the start: a note shows whole, never in a box to scroll.
    const field = useRef<HTMLTextAreaElement>(null);
    useLayoutEffect(() => {
        const el = field.current;
        if (el) {
            el.style.height = 'auto';
            el.style.height = `${el.scrollHeight + 2}px`;
        }
    }, [text]);

    const label = formatMessage({id: 'fusion.profile.note', defaultMessage: 'Note'});
    return (
        <label className={am('user-note')}>
            <span>
                {label}
                <small>{formatMessage({id: 'fusion.profile.notePrivate', defaultMessage: 'only you see it'})}</small>
            </span>
            <textarea
                ref={field}
                rows={1}
                value={text}
                maxLength={USER_NOTE_MAX}
                placeholder={formatMessage({id: 'fusion.profile.notePlaceholder', defaultMessage: 'Click to add a note'})}
                aria-label={label}
                onChange={(e) => setText(e.target.value)}
                onBlur={save}
                onKeyDown={(e) => {
                    // Enter saves; Shift+Enter starts a new line. Escape closes the card, keeping the note.
                    if (e.key === 'Enter' && !e.shiftKey) {
                        e.preventDefault();
                        e.currentTarget.blur();
                    }
                }}
            />
        </label>
    );
}
