// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import {defineMessage} from 'react-intl';
import type {MessageDescriptor} from 'react-intl';

import type {ChannelForumView, ChannelKindMode, ChannelKindType, ChannelPresetId, ChannelRootPosts} from '@mattermost/types/channels';

import type {IconName} from 'fusion/components/icon';

// The channel types and their presets (the design's §0 and §2), mirroring the server's model/channel_type.go: the
// type picker's segments, and each type's preset cards, in the order the picker shows them.

export type ChannelTypeCard = {
    id: ChannelKindType;
    icon: IconName;
};

// The type switch's three segments.
export const CHANNEL_TYPES: ChannelTypeCard[] = [
    {id: 'text', icon: 'hash'},
    {id: 'voice', icon: 'speaker'},
    {id: 'forum', icon: 'forum'},
];

export function channelTypeLabel(type: ChannelKindType): MessageDescriptor {
    switch (type) {
    case 'voice':
        return defineMessage({id: 'fusion.channelType.voice', defaultMessage: 'Voice', description: 'The Voice segment of the channel type switch, in the create-channel dialog and channel settings.'});
    case 'forum':
        return defineMessage({id: 'fusion.channelType.forum', defaultMessage: 'Forum', description: 'The Forum segment of the channel type switch, in the create-channel dialog and channel settings.'});
    default:
        return defineMessage({id: 'fusion.channelType.text', defaultMessage: 'Text', description: 'The Text segment of the channel type switch, in the create-channel dialog and channel settings.'});
    }
}

// The "Kind of…" header above each type's preset cards.
export function channelTypePresetsLabel(type: ChannelKindType): MessageDescriptor {
    switch (type) {
    case 'voice':
        return defineMessage({id: 'fusion.channelType.kindOfCall', defaultMessage: 'Kind of call', description: 'Header above the preset cards once Voice is picked as the channel type.'});
    case 'forum':
        return defineMessage({id: 'fusion.channelType.kindOfForum', defaultMessage: 'Kind of forum', description: 'Header above the preset cards once Forum is picked as the channel type.'});
    default:
        return defineMessage({id: 'fusion.channelType.kindOfChannel', defaultMessage: 'Kind of channel', description: 'Header above the preset cards once Text is picked as the channel type.'});
    }
}

export type ChannelPresetCard = {
    id: ChannelPresetId;
    type: ChannelKindType;
    mode: ChannelKindMode;
    root_posts: ChannelRootPosts;
    view?: ChannelForumView;
    label: MessageDescriptor;
    description: MessageDescriptor;
};

// The presets, in picker order (the design's §2 table).
export const CHANNEL_PRESETS: ChannelPresetCard[] = [
    {
        id: 'text_chat',
        type: 'text',
        mode: 'chat',
        root_posts: 'members',
        label: defineMessage({id: 'fusion.preset.textChat', defaultMessage: 'Chat', description: 'Name of the default text channel preset, on its card in the channel type picker.'}),
        description: defineMessage({id: 'fusion.preset.textChat.description', defaultMessage: 'Messages, files and threads', description: 'Subtitle of the Chat preset card: what the channel offers.'}),
    },
    {
        id: 'text_announcement',
        type: 'text',
        mode: 'chat',
        root_posts: 'moderators',
        label: defineMessage({id: 'fusion.preset.textAnnouncement', defaultMessage: 'Announcement', description: 'Name of the text channel preset where only moderators post, on its card in the channel type picker.'}),
        description: defineMessage({id: 'fusion.preset.textAnnouncement.description', defaultMessage: 'Moderators post; everyone replies in threads', description: 'Subtitle of the Announcement preset card: what the channel offers.'}),
    },
    {
        id: 'voice_talk',
        type: 'voice',
        mode: 'talk',
        root_posts: 'members',
        label: defineMessage({id: 'fusion.preset.voiceTalk', defaultMessage: 'Talk', description: 'Name of the default voice channel preset, on its card in the channel type picker.'}),
        description: defineMessage({id: 'fusion.preset.voiceTalk.description', defaultMessage: 'An always-open call; drop in and talk', description: 'Subtitle of the Talk preset card: what the channel offers.'}),
    },
    {
        id: 'voice_stage',
        type: 'voice',
        mode: 'stage',
        root_posts: 'members',
        label: defineMessage({id: 'fusion.preset.voiceStage', defaultMessage: 'Stage', description: 'Name of the voice channel preset with speakers and an audience, on its card in the channel type picker.'}),
        description: defineMessage({id: 'fusion.preset.voiceStage.description', defaultMessage: 'Speakers talk, the audience listens and asks to speak', description: 'Subtitle of the Stage preset card: what the channel offers.'}),
    },
    {
        id: 'forum_agora',
        type: 'forum',
        mode: 'discussion',
        root_posts: 'members',
        view: 'topics',
        label: defineMessage({id: 'fusion.preset.forumAgora', defaultMessage: 'Discussion', description: 'Name of the default forum preset, on its card in the channel type picker.'}),
        description: defineMessage({id: 'fusion.preset.forumAgora.description', defaultMessage: 'Members open topics and reply', description: 'Subtitle of the Discussion preset card: what the forum offers.'}),
    },
    {
        id: 'forum_qa',
        type: 'forum',
        mode: 'qa',
        root_posts: 'members',
        view: 'topics',
        label: defineMessage({id: 'fusion.preset.forumQa', defaultMessage: 'Q&A', description: 'Name of the question-and-answer forum preset, on its card in the channel type picker.'}),
        description: defineMessage({id: 'fusion.preset.forumQa.description', defaultMessage: 'Questions, voted answers, one accepted answer', description: 'Subtitle of the Q&A preset card: what the forum offers.'}),
    },
    {
        id: 'forum_ideas',
        type: 'forum',
        mode: 'ideas',
        root_posts: 'members',
        view: 'topics',
        label: defineMessage({id: 'fusion.preset.forumIdeas', defaultMessage: 'Ideas', description: 'Name of the ideas-board forum preset, on its card in the channel type picker.'}),
        description: defineMessage({id: 'fusion.preset.forumIdeas.description', defaultMessage: 'Members suggest and upvote; moderators set a status', description: 'Subtitle of the Ideas preset card: what the forum offers.'}),
    },
    {
        id: 'forum_announcement',
        type: 'forum',
        mode: 'discussion',
        root_posts: 'moderators',
        view: 'topics',
        label: defineMessage({id: 'fusion.preset.forumAnnouncement', defaultMessage: 'Announcements', description: 'Name of the forum preset where only moderators open topics, on its card in the channel type picker.'}),
        description: defineMessage({id: 'fusion.preset.forumAnnouncement.description', defaultMessage: 'Moderators open topics; members reply', description: 'Subtitle of the Announcements forum preset card: what the forum offers.'}),
    },
    {
        id: 'forum_imageboard',
        type: 'forum',
        mode: 'discussion',
        root_posts: 'members',
        view: 'imageboard',
        label: defineMessage({id: 'fusion.preset.forumImageboard', defaultMessage: 'Image board', description: 'Name of the image-board forum preset, on its card in the channel type picker.'}),
        description: defineMessage({id: 'fusion.preset.forumImageboard.description', defaultMessage: 'Each topic opens on its image and latest replies', description: 'Subtitle of the Image board preset card: what the forum offers.'}),
    },
];

export const CHANNEL_PRESET_IDS: ReadonlySet<ChannelPresetId> = new Set(CHANNEL_PRESETS.map((p) => p.id));

export function isChannelPresetId(id: string): id is ChannelPresetId {
    return CHANNEL_PRESET_IDS.has(id as ChannelPresetId);
}

export function channelPresetsOfType(type: ChannelKindType): ChannelPresetCard[] {
    return CHANNEL_PRESETS.filter((p) => p.type === type);
}

export function defaultPresetForType(type: ChannelKindType): ChannelPresetId {
    return channelPresetsOfType(type)[0].id;
}

export function channelPreset(id: ChannelPresetId): ChannelPresetCard | undefined {
    return CHANNEL_PRESETS.find((p) => p.id === id);
}

// presetForInfo finds which preset a channel's settings match, or '' when they're custom (mirrors the server's
// ChannelTypeInfo.Preset).
export function presetForInfo(info: {type: ChannelKindType; mode: ChannelKindMode; root_posts: ChannelRootPosts; view?: ChannelForumView}): ChannelPresetId | '' {
    const found = CHANNEL_PRESETS.find((p) => p.type === info.type && p.mode === info.mode && p.root_posts === info.root_posts && (p.type !== 'forum' || p.view === info.view));
    return found ? found.id : '';
}

// Channel settings' two fine-tuning controls (design §4.2), labelled by type.
export function rootPostsLabel(type: ChannelKindType): MessageDescriptor {
    return type === 'forum' ? defineMessage({id: 'fusion.channelType.whoOpensTopics', defaultMessage: 'Who can open topics', description: 'Channel settings Type tab: the setting choosing who may open a new topic in a forum.'}) : defineMessage({id: 'fusion.channelType.whoStarts', defaultMessage: 'Who can start conversations', description: 'Channel settings Type tab: the setting choosing who may start a new conversation in a text or voice channel.'});
}

export const ROOT_POSTS_OPTIONS: Array<{id: ChannelRootPosts; label: MessageDescriptor}> = [
    {id: 'members', label: defineMessage({id: 'fusion.channelType.members', defaultMessage: 'Members', description: 'Option of the who-starts-conversations setting: every member may.'})},
    {id: 'moderators', label: defineMessage({id: 'fusion.channelType.moderators', defaultMessage: 'Moderators', description: 'Option of the who-starts-conversations setting: only the channel\'s moderators may.'})},
];

export const FORUM_VIEW_OPTIONS: Array<{id: ChannelForumView; label: MessageDescriptor}> = [
    {id: 'topics', label: defineMessage({id: 'fusion.channelType.viewTopics', defaultMessage: 'Topics', description: 'Option of a forum\'s default view setting: the topic list.'})},
    {id: 'imageboard', label: defineMessage({id: 'fusion.channelType.viewImageboard', defaultMessage: 'Image board', description: 'Option of a forum\'s default view setting: the image board.'})},
];
