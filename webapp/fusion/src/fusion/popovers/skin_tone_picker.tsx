// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import React, {useState} from 'react';
import {useIntl} from 'react-intl';
import {useDispatch, useSelector} from 'react-redux';

import {setUserSkinTone} from 'actions/emoji_actions';
import {getUserSkinTone} from 'selectors/emojis';

import RenderEmoji from 'components/emoji/render_emoji';

import {am} from 'fusion/utils/class_names';

// The skin tones of the classic picker, lightest first, with the hand that shows each.
const TONES: Array<{value: string; emoji: string; label: {id: string; defaultMessage: string}}> = [
    {value: 'default', emoji: 'raised_hand_with_fingers_splayed', label: {id: 'emoji_skin.default', defaultMessage: 'Default skin tone'}},
    {value: '1F3FB', emoji: 'raised_hand_with_fingers_splayed_light_skin_tone', label: {id: 'emoji_skin.light_skin_tone', defaultMessage: 'Light skin tone'}},
    {value: '1F3FC', emoji: 'raised_hand_with_fingers_splayed_medium_light_skin_tone', label: {id: 'emoji_skin.medium_light_skin_tone', defaultMessage: 'Medium light skin tone'}},
    {value: '1F3FD', emoji: 'raised_hand_with_fingers_splayed_medium_skin_tone', label: {id: 'emoji_skin.medium_skin_tone', defaultMessage: 'Medium skin tone'}},
    {value: '1F3FE', emoji: 'raised_hand_with_fingers_splayed_medium_dark_skin_tone', label: {id: 'emoji_skin.medium_dark_skin_tone', defaultMessage: 'Medium dark skin tone'}},
    {value: '1F3FF', emoji: 'raised_hand_with_fingers_splayed_dark_skin_tone', label: {id: 'emoji_skin.dark_skin_tone', defaultMessage: 'Dark skin tone'}},
];

// SkinTonePicker is the picker's skin tone button: it shows a hand in your tone, and opens the tones to pick another.
export default function SkinTonePicker() {
    const {formatMessage} = useIntl();
    const dispatch = useDispatch();
    const current = useSelector(getUserSkinTone);
    const [open, setOpen] = useState(false);
    const tone = TONES.find((t) => t.value === current) || TONES[0];

    if (!open) {
        return (
            <button
                type='button'
                className={am('icon-btn', 'skin-btn')}
                title={formatMessage({id: 'fusion.picker.skinTone', defaultMessage: 'Skin tone: {tone}'}, {tone: formatMessage(tone.label)})}
                aria-label={formatMessage({id: 'fusion.picker.skinTone', defaultMessage: 'Skin tone: {tone}'}, {tone: formatMessage(tone.label)})}
                aria-expanded={false}
                onClick={() => setOpen(true)}
            >
                <RenderEmoji
                    emojiName={tone.emoji}
                    size={20}
                />
            </button>
        );
    }
    return (
        <div
            className={am('skin-tones')}
            role='radiogroup'
            aria-label={formatMessage({id: 'fusion.picker.skinTones', defaultMessage: 'Skin tones'})}
        >
            {TONES.map((t) => (
                <button
                    key={t.value}
                    type='button'
                    role='radio'
                    aria-checked={t.value === tone.value}
                    className={am({on: t.value === tone.value})}
                    title={formatMessage(t.label)}
                    aria-label={formatMessage(t.label)}
                    onClick={() => {
                        if (t.value !== current) {
                            dispatch(setUserSkinTone(t.value));
                        }
                        setOpen(false);
                    }}
                >
                    <RenderEmoji
                        emojiName={t.emoji}
                        size={20}
                    />
                </button>
            ))}
        </div>
    );
}
