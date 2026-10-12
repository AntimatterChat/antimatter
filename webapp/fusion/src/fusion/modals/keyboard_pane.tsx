// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useEffect, useMemo, useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {savePreferences} from 'mattermost-redux/actions/preferences';
import {getCurrentUserId} from 'mattermost-redux/selectors/entities/users';

import {CATEGORY_FUSION_KEYS, KEY_ACTIONS, NAME_KEYBINDS, comboLabel, comboOf, getSavedKeybinds, parseKeybinds, withDefaults} from 'fusion/shell/keybinds';
import type {KeyAction} from 'fusion/shell/keybinds';
import {am} from 'fusion/utils/class_names';
import {openKeyboardShortcuts} from 'fusion/utils/modals';

function Combo({combo}: {combo: string}) {
    const {formatMessage} = useIntl();
    if (!combo) {
        return <span className={am('key-off')}>{formatMessage({id: 'fusion.keys.off', defaultMessage: 'Off'})}</span>;
    }
    return (
        <span className={am('key-combo')}>
            {comboLabel(combo).map((part) => <kbd key={part}>{part}</kbd>)}
        </span>
    );
}

// KeyboardPane changes the Fusion UI's own keyboard shortcuts: press the new keys after "Change", Escape to cancel,
// Backspace to turn one off. The classic web app's other shortcuts stay as they are.
export default function KeyboardPane() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const userId = useSelector(getCurrentUserId);
    const saved = useSelector(getSavedKeybinds);
    const custom = useMemo(() => parseKeybinds(saved), [saved]);
    const binds = useMemo(() => withDefaults(custom), [custom]);
    const [recording, setRecording] = useState<KeyAction | null>(null);

    const save = (next: Partial<Record<KeyAction, string>>) => dispatch(savePreferences(userId, [{user_id: userId, category: CATEGORY_FUSION_KEYS, name: NAME_KEYBINDS, value: JSON.stringify(next)}]));

    useEffect(() => {
        if (!recording) {
            return undefined;
        }
        const onKeyDown = (e: KeyboardEvent) => {
            e.preventDefault();
            e.stopImmediatePropagation();
            if (e.key === 'Escape') {
                setRecording(null);
                return;
            }
            if (e.key === 'Backspace' || e.key === 'Delete') {
                save({...custom, [recording]: ''});
                setRecording(null);
                return;
            }
            const combo = comboOf(e);
            if (combo) {
                save({...custom, [recording]: combo});
                setRecording(null);
            }
        };

        // On the window, ahead of the settings dialog's own keys: Escape stops recording, not the dialog.
        window.addEventListener('keydown', onKeyDown, true);
        return () => window.removeEventListener('keydown', onKeyDown, true);
        // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [recording]);

    return (
        <>
            <h2>{formatMessage({id: 'fusion.settings.keyboard', defaultMessage: 'Keyboard'})}</h2>
            <p className={am('lead')}>{formatMessage({id: 'fusion.settings.keyboardLead', defaultMessage: 'Change the shortcuts of Antimatter\'s own actions. Click Change, then press the new keys; Backspace turns a shortcut off.'})}</p>
            {KEY_ACTIONS.map((action) => {
                const combo = binds[action.id];
                const clash = combo && KEY_ACTIONS.some((other) => other.id !== action.id && binds[other.id] === combo);
                return (
                    <div
                        key={action.id}
                        className={am('toggle-row')}
                    >
                        <div className={am('txt')}>
                            <b>{formatMessage(action.label)}</b>
                            {clash && <span className={am('field-error')}>{formatMessage({id: 'fusion.keys.clash', defaultMessage: 'Another action uses these keys too'})}</span>}
                        </div>
                        {recording === action.id ? <span className={am('key-rec')}>{formatMessage({id: 'fusion.keys.press', defaultMessage: 'Press the keys…'})}</span> : <Combo combo={combo}/>}
                        <button
                            type='button'
                            className={am('btn')}
                            onClick={() => setRecording(recording === action.id ? null : action.id)}
                        >
                            {recording === action.id ? formatMessage({id: 'fusion.settings.cancel', defaultMessage: 'Cancel'}) : formatMessage({id: 'fusion.keys.change', defaultMessage: 'Change'})}
                        </button>
                        {action.id in custom && recording !== action.id && (
                            <button
                                type='button'
                                className={am('btn')}
                                title={formatMessage({id: 'fusion.keys.resetTitle', defaultMessage: 'Back to {combo}'}, {combo: comboLabel(action.combo).join('+')})}
                                onClick={() => {
                                    const next = {...custom};
                                    delete next[action.id];
                                    save(next);
                                }}
                            >
                                {formatMessage({id: 'fusion.keys.reset', defaultMessage: 'Reset'})}
                            </button>
                        )}
                    </div>
                );
            })}
            <div className={am('modal-actions')}>
                <button
                    type='button'
                    className={am('btn')}
                    onClick={() => dispatch(openKeyboardShortcuts())}
                >
                    {formatMessage({id: 'fusion.keys.all', defaultMessage: 'All keyboard shortcuts…'})}
                </button>
            </div>
        </>
    );
}
