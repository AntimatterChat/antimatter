// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

// Server-owned props on a topic's root post (design §3.3), written only by PATCH /posts/{root_id}/topic: a client
// or app can't set them on create, update or patch (model.PostPropsServerOwned).
export const ACCEPTED_ANSWER_PROP = 'am_accepted_answer';
export const TOPIC_STATUS_PROP = 'am_topic_status';
