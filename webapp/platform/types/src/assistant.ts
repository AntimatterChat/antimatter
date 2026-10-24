// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Antimatter: the assistant (stage 1 of the ambient assistant), at /api/v4/assistant. A personal feed of cards for
// the conversation a person has the assistant open on: insights (a tool that reads, run as the person) and actions
// (a tool that changes something, offered to one person at a time and never run on its own). The server sends cards
// to their owner only (assistant_card_added, assistant_card_updated) and tells everyone with an action's card where
// its offer stands (assistant_offer_updated).

export type AssistantEngine = 'rules' | 'model';
export type AssistantCardKind = 'insight' | 'action';
export type AssistantCardStatus = 'new' | 'dismissed' | 'posted';
export type AssistantOfferStatus = 'open' | 'done' | 'expired';

// The preferences of the assistant: per channel (named by its ID) 'on' or 'off', and the muted tools (a JSON list).
export const PREFERENCE_CATEGORY_ASSISTANT = 'assistant';
export const PREFERENCE_NAME_ASSISTANT_MUTED_TOOLS = 'muted_tools';

// The prop of a message posted from a card: the card's ID.
export const POST_PROP_ASSISTANT_CARD = 'assistant_card';

export type AssistantEntityMatch = {
    type: string;
    value: string;
    text: string;
};

// Why a card appeared: the pattern and the words that matched (rules), or the model's probabilities (model).
export type AssistantWhy = {
    engine: AssistantEngine;
    pattern?: string;
    words?: string[];
    entities?: AssistantEntityMatch[];
    probabilities?: Record<string, number>;
};

// An action's offer as the person sees it.
export type AssistantOfferView = {
    id: string;
    status: AssistantOfferStatus;

    // Whom it's offered to now; '' for nobody.
    holder_id: string;
    offered_to_me: boolean;
    can_run: boolean;

    // When it goes to the next person if the holder neither runs nor dismisses it (milliseconds; 0: it doesn't).
    moves_at: number;
    done_by?: string;
    done_at?: number;
};

export type AssistantCard = {
    id: string;
    user_id: string;
    channel_id: string;
    kind: AssistantCardKind;
    tool: string;
    tool_title: string;
    arguments: Record<string, unknown>;
    why: AssistantWhy;

    // The triggering message first.
    post_ids: string[];
    dedupe_key: string;

    // What the tool gave (insights only).
    result?: string;
    status: AssistantCardStatus;
    pinned: boolean;
    offer_id?: string;
    offer?: AssistantOfferView;
    create_at: number;
    update_at: number;
};

export type AssistantFeed = {
    channel_id: string;

    // Whether the assistant works in the channel (the server's and the community's settings), and the person's own
    // switch for it.
    enabled: boolean;
    on: boolean;
    engine: AssistantEngine;

    // Pinned first, then the newest.
    cards: AssistantCard[];
};

export type AssistantPresence = {
    following: boolean;
    ttl_seconds: number;
    heartbeat_seconds: number;
};

export type AssistantMutedTools = {
    tools: string[];
};

export type AssistantRunResult = {
    card: AssistantCard;

    // False until actions run (stage 3): the offer is claimed and closed.
    executed: boolean;
};

export type AssistantCommunitySettings = {

    // null: the server's setting.
    enabled: boolean | null;
    engine: AssistantEngine | '';
    connection_id: string;
    model: string;
};

export type AssistantCommunitySettingsInfo = {
    team_id: string;
    server_enabled: boolean;
    may_override: boolean;
    settings: AssistantCommunitySettings;
};

// The events' data.
export type AssistantCardEventData = {
    channel_id: string;
    card: AssistantCard;
};

export type AssistantOfferEventData = {
    channel_id: string;
    card_id: string;
    offer: AssistantOfferView;
};

// The assistant's server settings (AssistantSettings).
export type AssistantSettings = {
    Enable: boolean;
    Engine: AssistantEngine;
    AllowCommunityOverride: boolean;
    ConnectionId: string;
    Model: string;
    ThresholdPercent: number;
    DecisionsPerUserHour: number;
    DecisionsPerChannelHour: number;
    CardsPerUserHour: number;
    DebounceSeconds: number;
    OfferTimeoutSeconds: number;
    AllowedTools: string[];
    Patterns: AssistantPattern[] | null;
    SeedEntities: AssistantEntity[] | null;
    DictionarySources: AssistantDictionarySource[] | null;
};

export type AssistantPattern = {
    id: string;
    kind: AssistantCardKind;
    match: string[];
    tools: string[];
    arguments?: Record<string, string>;
};

export type AssistantEntity = {
    type: string;
    value: string;
    aliases?: string[];
};

export type AssistantDictionarySource = {
    tool: string;
    type: string;
    field: string;
    arguments?: Record<string, unknown>;
};
