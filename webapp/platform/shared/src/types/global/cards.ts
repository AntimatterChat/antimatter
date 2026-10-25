// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {ComponentType} from 'react';

import type {MessageAttachment} from '@mattermost/types/message_attachments';

// Plugin-facing contract for window.WebappUtils.cards (ASSISTANT_DISPLAY.md #2). Hand-authored, not derived: the
// web app checks at build time that the real component stays assignable to this contract in published_cards.ts,
// so drift breaks the build, not apps.

export type PublishedAttachmentsCardProps = {
    attachments: MessageAttachment[];

    // One plain-text line: the card's accessible group label (so a screen reader announces it even for a
    // chart-only result), shown nowhere else by this renderer.
    summary: string;
};

export type PublishedCardsUtils = {
    // Draws a result's attachments with the same look as a posted message's (chart, fields, ...), for content
    // that isn't a post itself: an assistant card, shown with a channel id but no post of its own. Actions are
    // always hidden, whatever the attachments carry.
    MessageAttachmentCard: ComponentType<PublishedAttachmentsCardProps>;
};
