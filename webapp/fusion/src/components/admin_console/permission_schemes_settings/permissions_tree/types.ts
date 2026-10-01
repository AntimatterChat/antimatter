// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

export type Permissions = Array<string | Group | Permission>;

export type Permission = {
    id: string;
    combined?: boolean;
    permissions: string[];
};
export type Group = {
    id: string;
    permissions: Array<Permission | string>;
};

export type AdditionalValues = Record<string, Record<string, any>>;
