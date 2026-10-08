// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

import type {Channel} from './channels';
import type {Team} from './teams';
import type {UserProfile} from './users';
import type {IDMappedObjects, RelationOneToOne} from './utilities';

export type ChannelCategoryType = 'favorites' | 'channels' | 'direct_messages' | 'custom' | 'managed';

export enum CategorySorting {
    Alphabetical = 'alpha',
    Default = '', // behaves the same as manual
    Recency = 'recent',
    Manual = 'manual',
}

export type ChannelCategory = {
    id: string;
    user_id: UserProfile['id'];
    team_id: Team['id'];
    type: ChannelCategoryType;
    display_name: string;
    sorting: CategorySorting;
    channel_ids: Array<Channel['id']>;
    muted: boolean;
    collapsed: boolean;
};

export type OrderedChannelCategories = {
    categories: ChannelCategory[];
    order: string[];
};

/**
 * A sidebar category an admin sets up for everyone in a team (Antimatter): channels are filed in it by name, as managed
 * categories. People can move the channels of an open category into their own; a locked one keeps them.
 */
export type ServerCategory = {
    team_id: string;
    name: string;
    sort_order: number;
    locked: boolean;
    create_at: number;
    update_at: number;
};

/** Replaces a team's server categories, in order; renames map old names to new ones. */
export type ServerCategoriesUpdate = {
    categories: Array<Pick<ServerCategory, 'name' | 'locked'>>;
    renames?: Record<string, string>;
};

export type ChannelCategoriesState = {
    byId: IDMappedObjects<ChannelCategory>;
    orderByTeam: RelationOneToOne<Team, Array<ChannelCategory['id']>>;
    managedCategoryMappings: RelationOneToOne<Team, Record<Channel['id'], string>>;
    serverCategories: RelationOneToOne<Team, ServerCategory[]>;
};
