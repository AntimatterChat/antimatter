// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import {MessageAttachmentCard} from 'fusion/messages/content/cards';

import {publishedCardsUtils} from './published_cards';

describe('publishedCardsUtils', () => {
    test('exposes Fusion\'s own attachment card renderer', () => {
        expect(publishedCardsUtils.MessageAttachmentCard).toBe(MessageAttachmentCard);
        expect(typeof publishedCardsUtils.MessageAttachmentCard).toBe('function');
    });
});
