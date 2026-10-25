// Copyright (c) 2026-present Antimatter contributors.
// See LICENSE.txt for license information.

import type {ComponentProps} from 'react';

import type {PublishedAttachmentsCardProps, PublishedCardsUtils} from '@mattermost/shared/types/global';

import {MessageAttachmentCard} from 'fusion/messages/content/cards';

// Fails the build here if MessageAttachmentCard's real props drift from the published contract, instead of
// silently breaking apps that render cards from window.WebappUtils.cards (ASSISTANT_DISPLAY.md #2).
type ContractHonored<T extends ComponentProps<typeof MessageAttachmentCard>> = T;

// eslint-disable-next-line @typescript-eslint/no-unused-vars
type AssertPublishedCardsContract = ContractHonored<PublishedAttachmentsCardProps>;

export const publishedCardsUtils: PublishedCardsUtils = {
    MessageAttachmentCard,
};
