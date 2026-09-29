// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"net/http"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
)

// isBurnOnReadEnabled reports whether the burn-on-read feature is active, i.e.
// both the feature flag and the service setting are turned on.
func (a *App) isBurnOnReadEnabled() bool {
	return a.Config().FeatureFlags.BurnOnRead && model.SafeDereference(a.Config().ServiceSettings.EnableBurnOnRead)
}

// filterBurnOnReadPosts filters out burn-on-read posts from a PostList.
// This should be used for contexts where burn-on-read posts should not appear (e.g., search results).
func (a *App) filterBurnOnReadPosts(postList *model.PostList) *model.AppError {
	if postList == nil || postList.Posts == nil || len(postList.Posts) == 0 {
		return nil
	}

	// Check if burn-on-read feature is enabled
	if !a.Config().FeatureFlags.BurnOnRead || !model.SafeDereference(a.Config().ServiceSettings.EnableBurnOnRead) {
		// Feature is not enabled, no need to filter
		return nil
	}

	// Collect burn-on-read post IDs
	var burnOnReadPostIDs []string
	for postID, post := range postList.Posts {
		if post.Type == model.PostTypeBurnOnRead {
			burnOnReadPostIDs = append(burnOnReadPostIDs, postID)
		}
	}

	// If no burn-on-read posts found, nothing to filter
	if len(burnOnReadPostIDs) == 0 {
		return nil
	}

	// Remove burn-on-read posts from the list
	for _, postID := range burnOnReadPostIDs {
		a.removePostFromList(postList, postID)
	}

	// Filter Order slice directly to ensure all burn-on-read posts are removed
	filteredOrder := make([]string, 0, len(postList.Order))
	for _, postID := range postList.Order {
		if post, exists := postList.Posts[postID]; exists && post.Type != model.PostTypeBurnOnRead {
			filteredOrder = append(filteredOrder, postID)
		}
	}
	postList.Order = filteredOrder

	// Clear BurnOnReadPosts map as burn-on-read posts should not appear
	postList.BurnOnReadPosts = make(map[string]*model.Post)

	// Update NextPostId and PrevPostId if they point to removed posts
	if postList.NextPostId != "" {
		if _, exists := postList.Posts[postList.NextPostId]; !exists {
			postList.NextPostId = ""
		}
	}
	if postList.PrevPostId != "" {
		if _, exists := postList.Posts[postList.PrevPostId]; !exists {
			postList.PrevPostId = ""
		}
	}

	return nil
}

// revealSingleBurnOnReadPost reveals a single burn-on-read post for a user.
// If the post is not a burn-on-read post, it returns the post unchanged.
// If the post is expired or inaccessible, it returns an error.
func (a *App) revealSingleBurnOnReadPost(rctx request.CTX, post *model.Post, userID string) (*model.Post, *model.AppError) {
	if post == nil {
		return nil, model.NewAppError("revealSingleBurnOnReadPost", "app.post.get.app_error", nil, "", http.StatusBadRequest)
	}

	// If not a burn-on-read post, return as-is
	if post.Type != model.PostTypeBurnOnRead {
		return post, nil
	}

	// Check if burn-on-read feature is enabled
	if !a.Config().FeatureFlags.BurnOnRead || !model.SafeDereference(a.Config().ServiceSettings.EnableBurnOnRead) {
		// Feature is not enabled, return post as-is
		return post, nil
	}

	tmpPostList := model.NewPostList()
	tmpPostList.AddPost(post)

	postList, appErr := a.revealBurnOnReadPostsForUser(rctx, tmpPostList, userID)
	if appErr != nil {
		return nil, appErr
	}

	revealedPost, ok := postList.Posts[post.Id]
	if !ok {
		return nil, model.NewAppError("revealSingleBurnOnReadPost", "app.post.get.app_error", nil, "", http.StatusNotFound)
	}

	return revealedPost, nil
}

// revealBurnOnReadPostsForUser processes burn-on-read posts in a post list for a specific user,
// revealing posts that the user has access to and handling expired receipts.
func (a *App) revealBurnOnReadPostsForUser(rctx request.CTX, postList *model.PostList, userID string) (*model.PostList, *model.AppError) {
	if postList == nil || postList.BurnOnReadPosts == nil || len(postList.BurnOnReadPosts) == 0 {
		return postList, nil
	}

	// Check if burn-on-read feature is enabled
	if !a.Config().FeatureFlags.BurnOnRead || !model.SafeDereference(a.Config().ServiceSettings.EnableBurnOnRead) {
		// Feature is not enabled, return postList as-is
		return postList, nil
	}

	for _, post := range postList.BurnOnReadPosts {
		if post.DeleteAt > 0 {
			continue
		}

		// If user is the author, reveal the post with recipients
		if post.UserId == userID {
			if err := a.revealPostForAuthor(rctx, postList, post); err != nil {
				return nil, err
			}
			continue
		}

		// Get user's read receipt for this post
		receipt, err := a.getUserReadReceipt(rctx, post.Id, userID)
		if err != nil {
			return nil, err
		}

		// If no receipt exists, show unrevealed message
		if receipt == nil {
			a.setUnrevealedPost(postList, post.Id)
			continue
		}

		// If receipt expired, remove post from list
		if a.isReceiptExpired(receipt) {
			a.removePostFromList(postList, post.Id)
			continue
		}

		// Reveal post with expiration metadata
		if err := a.revealPostForUser(rctx, postList, post, receipt); err != nil {
			return nil, err
		}
	}

	return postList, nil
}
