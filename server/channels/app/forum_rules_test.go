// Copyright (c) 2015-present Mattermost, Inc. All Rights Reserved.
// See LICENSE.txt for license information.

package app

import (
	"encoding/json"
	"maps"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app/imports"
)

// Antimatter forums, rule 1: a forum takes only topics (a title and a first post), whoever posts and however; and
// the server-owned props of a topic (its accepted answer and status) can't be forged by posting or editing. A
// regression either lets untitled messages into forums, or lets anyone mark their own answer accepted or their idea
// done, so each way in is spelt out.

const titleRequired = "app.post.forum.title_required.app_error"

func requireBadRequest(t *testing.T, appErr *model.AppError, id string, msgAndArgs ...any) {
	t.Helper()
	require.NotNil(t, appErr, msgAndArgs...)
	assert.Equal(t, id, appErr.Id, msgAndArgs...)
	assert.Equal(t, http.StatusBadRequest, appErr.StatusCode, msgAndArgs...)
}

type forumFixture struct {
	th     *TestHelper
	forum  *model.Channel
	member *model.User
	topic  *model.Post // a topic of the forum, the member's
}

func setupForum(t *testing.T, preset string) *forumFixture {
	th := Setup(t).InitBasic(t)
	f := &forumFixture{th: th, forum: createTypedChannel(t, th, preset), member: th.BasicUser}
	th.AddUserToChannel(t, th.BasicUser, f.forum)
	th.AddUserToChannel(t, th.BasicUser2, f.forum)
	f.topic = f.post(t, f.member.Id, "", "First topic")
	return f
}

// post makes a post in the forum as the REST API does, and fails the test if it's refused.
func (f *forumFixture) post(t *testing.T, userID, rootID, title string) *model.Post {
	t.Helper()
	post, appErr := f.tryPost(userID, rootID, title, nil, model.CreatePostFlags{})
	require.Nil(t, appErr)
	return post
}

func (f *forumFixture) tryPost(userID, rootID, title string, props model.StringInterface, flags model.CreatePostFlags) (*model.Post, *model.AppError) {
	post := &model.Post{UserId: userID, ChannelId: f.forum.Id, RootId: rootID, Message: "hello " + model.NewId(), Props: model.StringInterface{}}
	for k, v := range props {
		post.AddProp(k, v)
	}
	if title != "" {
		post.AddProp(model.PostPropsThreadTitle, title)
	}
	saved, _, appErr := f.th.App.CreatePost(f.th.Context, post, f.forum, flags)
	return saved, appErr
}

func TestForumTakesOnlyTopics(t *testing.T) {
	mainHelper.Parallel(t)
	f := setupForum(t, model.ChannelPresetForumAgora)
	th := f.th

	t.Run("a root post needs a title; a reply doesn't, and loses one", func(t *testing.T) {
		_, _, appErr := th.App.CreatePostAsUser(th.Context, &model.Post{UserId: f.member.Id, ChannelId: f.forum.Id, Message: "No title"}, "", false)
		requireBadRequest(t, appErr, titleRequired, "the REST API's path")
		_, appErr = f.tryPost(f.member.Id, "", "   ", nil, model.CreatePostFlags{})
		requireBadRequest(t, appErr, titleRequired, "blank")
		_, appErr = f.tryPost(f.member.Id, "", strings.Repeat("t", model.ForumTopicTitleMaxRunes+1), nil, model.CreatePostFlags{})
		requireBadRequest(t, appErr, "app.post.forum.title_too_long.app_error")
		_, appErr = f.tryPost(f.member.Id, "", "", model.StringInterface{model.PostPropsThreadTitle: 7}, model.CreatePostFlags{})
		requireBadRequest(t, appErr, titleRequired, "a title that isn't text")

		topic := f.post(t, f.member.Id, "", "  Beam alignment  ")
		assert.Equal(t, "Beam alignment", topic.GetProp(model.PostPropsThreadTitle), "kept trimmed")

		reply := f.post(t, th.BasicUser2.Id, topic.Id, "")
		assert.Nil(t, reply.GetProp(model.PostPropsThreadTitle))
		titled := f.post(t, th.BasicUser2.Id, topic.Id, "Not a topic")
		assert.Nil(t, titled.GetProp(model.PostPropsThreadTitle), "a reply's title is dropped")
	})

	t.Run("an incoming webhook", func(t *testing.T) {
		_, appErr := th.App.CreateWebhookPost(th.Context, f.member.Id, f.forum, "From the hook", "", "", "", model.StringInterface{}, "", "", nil, false)
		requireBadRequest(t, appErr, titleRequired)
		_, appErr = th.App.CreateWebhookPost(th.Context, f.member.Id, f.forum, "From the hook", "", "", "", model.StringInterface{model.PostPropsThreadTitle: "Build failed"}, "", "", nil, false)
		require.Nil(t, appErr, "a hook that names its topic")
		_, appErr = th.App.CreateWebhookPost(th.Context, f.member.Id, f.forum, "A reply from the hook", "", "", "", model.StringInterface{}, "", f.topic.Id, nil, false)
		require.Nil(t, appErr)
	})

	t.Run("a slash command's answer", func(t *testing.T) {
		args := &model.CommandArgs{UserId: f.member.Id, ChannelId: f.forum.Id, TeamId: f.forum.TeamId, Command: "/weather"}
		_, appErr := th.App.HandleCommandResponsePost(th.Context, &model.Command{}, args, &model.CommandResponse{ResponseType: model.CommandResponseTypeInChannel, Text: "Sunny"}, false)
		requireBadRequest(t, appErr, titleRequired, "an integration's answer")
		_, appErr = th.App.HandleCommandResponsePost(th.Context, &model.Command{}, args, &model.CommandResponse{ResponseType: model.CommandResponseTypeInChannel, Text: "Sunny"}, true)
		requireBadRequest(t, appErr, titleRequired, "a built-in command's answer")
		_, appErr = th.App.HandleCommandResponsePost(th.Context, &model.Command{}, args, &model.CommandResponse{ResponseType: model.CommandResponseTypeInChannel, Text: "Sunny", Props: model.StringInterface{model.PostPropsThreadTitle: "Weather"}}, false)
		require.Nil(t, appErr, "an answer that names its topic")
	})

	t.Run("a bot", func(t *testing.T) {
		bot := th.CreateBot(t)
		botUser, appErr := th.App.GetUser(th.Context, bot.UserId)
		require.Nil(t, appErr)
		th.LinkUserToTeam(t, botUser, th.BasicTeam)
		th.AddUserToChannel(t, botUser, f.forum)
		_, appErr = f.tryPost(bot.UserId, "", "", nil, model.CreatePostFlags{})
		requireBadRequest(t, appErr, titleRequired)
		_, appErr = f.tryPost(bot.UserId, f.topic.Id, "", nil, model.CreatePostFlags{})
		require.Nil(t, appErr)
	})

	t.Run("the AI post tool", func(t *testing.T) {
		agent := th.CreateBot(t)
		args, err := json.Marshal(map[string]string{"channel_id": f.forum.Id, "message": "Posted for them"})
		require.NoError(t, err)
		call := &aiToolCall{agent: &model.AIAgent{UserId: agent.UserId}, user: f.member, channel: f.forum, teamID: f.forum.TeamId, arguments: args}
		_, err = th.App.aiToolPostMessage(th.Context, call)
		require.Error(t, err, "it can't name a topic, so it can't open one")

		replyArgs, err := json.Marshal(map[string]string{"channel_id": f.forum.Id, "message": "A reply for them", "root_id": f.topic.Id})
		require.NoError(t, err)
		call.arguments = replyArgs
		_, err = th.App.aiToolPostMessage(th.Context, call)
		require.NoError(t, err)
	})

	t.Run("apps, through the plugin API too", func(t *testing.T) {
		_, appErr := f.tryPost(f.member.Id, "", "", nil, model.CreatePostFlags{FromPlugin: true})
		requireBadRequest(t, appErr, titleRequired, "apps are trusted with rights, not with the forum's structure")

		api := NewPluginAPI(th.App, th.Context, &model.Manifest{Id: "some.app"})
		_, appErr = api.CreatePost(&model.Post{UserId: f.member.Id, ChannelId: f.forum.Id, Message: "From an app"})
		requireBadRequest(t, appErr, titleRequired)
		post, appErr := api.CreatePost(&model.Post{UserId: f.member.Id, ChannelId: f.forum.Id, Message: "From an app", Props: model.StringInterface{model.PostPropsThreadTitle: "Daily digest"}})
		require.Nil(t, appErr)
		assert.Equal(t, "Daily digest", post.GetProp(model.PostPropsThreadTitle))
		_, appErr = api.CreatePost(&model.Post{UserId: f.member.Id, ChannelId: f.forum.Id, RootId: f.topic.Id, Message: "An app's reply"})
		require.Nil(t, appErr)
	})

	t.Run("the server's own messages", func(t *testing.T) {
		_, _, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: f.member.Id, ChannelId: f.forum.Id, Type: model.PostTypeJoinChannel, Message: "joined"}, f.forum, model.CreatePostFlags{})
		require.Nil(t, appErr)
	})

	t.Run("other channels are as before", func(t *testing.T) {
		_, _, appErr := th.App.CreatePost(th.Context, &model.Post{UserId: f.member.Id, ChannelId: th.BasicChannel.Id, Message: "Untitled"}, th.BasicChannel, model.CreatePostFlags{})
		require.Nil(t, appErr)
		dm, appErr := th.App.GetOrCreateDirectChannel(th.Context, th.BasicUser.Id, th.BasicUser2.Id)
		require.Nil(t, appErr)
		_, _, appErr = th.App.CreatePost(th.Context, &model.Post{UserId: f.member.Id, ChannelId: dm.Id, Message: "Untitled"}, dm, model.CreatePostFlags{})
		require.Nil(t, appErr)
	})
}

func TestForumTopicEditsKeepTheTitle(t *testing.T) {
	mainHelper.Parallel(t)
	f := setupForum(t, model.ChannelPresetForumAgora)
	th := f.th
	th.Context = th.Context.WithSession(&model.Session{UserId: f.member.Id, Id: model.NewId()}).(*request.Context)

	t.Run("an edit without a title keeps it", func(t *testing.T) {
		edit := f.topic.Clone()
		edit.Message = "Edited by a client that knows nothing of titles"
		edit.SetProps(model.StringInterface{})
		updated, _, appErr := th.App.UpdatePost(th.Context, edit, nil)
		require.Nil(t, appErr)
		assert.Equal(t, "First topic", updated.GetProp(model.PostPropsThreadTitle))

		updated, _, appErr = th.App.PatchPost(th.Context, f.topic.Id, &model.PostPatch{Props: &model.StringInterface{model.PostPropsThreadTitle: "  "}}, nil)
		require.Nil(t, appErr)
		assert.Equal(t, "First topic", updated.GetProp(model.PostPropsThreadTitle), "nor can a blank one erase it")

		api := NewPluginAPI(th.App, th.Context, &model.Manifest{Id: "some.app"})
		appEdit := f.topic.Clone()
		appEdit.SetProps(model.StringInterface{"app": "x"})
		updated, appErr = api.UpdatePost(appEdit)
		require.Nil(t, appErr)
		assert.Equal(t, "First topic", updated.GetProp(model.PostPropsThreadTitle), "nor an app")
	})

	t.Run("a new title renames the topic", func(t *testing.T) {
		before, err := th.App.Srv().Store().Post().GetSingle(th.Context, f.topic.Id, false)
		require.NoError(t, err)
		updated, _, appErr := th.App.PatchPost(th.Context, f.topic.Id, &model.PostPatch{Props: &model.StringInterface{model.PostPropsThreadTitle: " Renamed "}}, nil)
		require.Nil(t, appErr)
		assert.Equal(t, "Renamed", updated.GetProp(model.PostPropsThreadTitle))
		assert.Equal(t, before.EditAt, updated.EditAt, "a rename alone isn't an edit of the message")

		_, _, appErr = th.App.PatchPost(th.Context, f.topic.Id, &model.PostPatch{Props: &model.StringInterface{model.PostPropsThreadTitle: strings.Repeat("t", model.ForumTopicTitleMaxRunes+1)}}, nil)
		requireBadRequest(t, appErr, "app.post.forum.title_too_long.app_error")
	})

	t.Run("a reply edited with a title still has none", func(t *testing.T) {
		reply := f.post(t, f.member.Id, f.topic.Id, "")
		updated, _, appErr := th.App.PatchPost(th.Context, reply.Id, &model.PostPatch{Props: &model.StringInterface{model.PostPropsThreadTitle: "Sneaky"}}, nil)
		require.Nil(t, appErr)
		assert.Nil(t, updated.GetProp(model.PostPropsThreadTitle))
	})
}

// RBAC regression: am_accepted_answer and am_topic_status (and its _by and _at) are the server's: posting, updating,
// patching and apps drop what they send and keep what's stored.
func TestForumServerOwnedPropsCannotBeForged(t *testing.T) {
	mainHelper.Parallel(t)
	f := setupForum(t, model.ChannelPresetForumQA)
	th := f.th
	th.Context = th.Context.WithSession(&model.Session{UserId: f.member.Id, Id: model.NewId()}).(*request.Context)
	forged := model.StringInterface{
		model.PostPropsAcceptedAnswer: model.NewId(),
		model.PostPropsTopicStatus:    model.TopicStatusDone,
		model.PostPropsTopicStatusBy:  f.member.Id,
		model.PostPropsTopicStatusAt:  1,
	}
	requireNoServerOwned := func(t *testing.T, post *model.Post, msgAndArgs ...any) {
		t.Helper()
		for _, key := range model.PostPropsServerOwned {
			assert.Nil(t, post.GetProp(key), msgAndArgs...)
		}
	}

	t.Run("on create, from every source", func(t *testing.T) {
		post, appErr := f.tryPost(f.member.Id, "", "Forged", forged, model.CreatePostFlags{})
		require.Nil(t, appErr)
		requireNoServerOwned(t, post, "a client")
		stored, err := th.App.Srv().Store().Post().GetSingle(th.Context, post.Id, false)
		require.NoError(t, err)
		requireNoServerOwned(t, stored, "nor stored")

		post, appErr = f.tryPost(f.member.Id, "", "Forged", forged, model.CreatePostFlags{FromPlugin: true})
		require.Nil(t, appErr)
		requireNoServerOwned(t, post, "an app")

		hookProps := maps.Clone(forged)
		hookProps[model.PostPropsThreadTitle] = "Forged"
		post, appErr = th.App.CreateWebhookPost(th.Context, f.member.Id, f.forum, "From the hook", "", "", "", hookProps, "", "", nil, false)
		require.Nil(t, appErr)
		requireNoServerOwned(t, post, "a webhook")

		reply, appErr := f.tryPost(f.member.Id, f.topic.Id, "", forged, model.CreatePostFlags{})
		require.Nil(t, appErr)
		requireNoServerOwned(t, reply, "a reply")
	})

	// What PatchForumTopic would have written.
	reply := f.post(t, th.BasicUser2.Id, f.topic.Id, "")
	stored := f.topic.Clone()
	stored.AddProp(model.PostPropsAcceptedAnswer, reply.Id)
	stored.AddProp(model.PostPropsTopicStatus, model.TopicStatusPlanned)
	_, err := th.App.Srv().Store().Post().Overwrite(th.Context, stored)
	require.NoError(t, err)
	requireKept := func(t *testing.T, post *model.Post, msgAndArgs ...any) {
		t.Helper()
		assert.Equal(t, reply.Id, post.GetProp(model.PostPropsAcceptedAnswer), msgAndArgs...)
		assert.Equal(t, model.TopicStatusPlanned, post.GetProp(model.PostPropsTopicStatus), msgAndArgs...)
		assert.Nil(t, post.GetProp(model.PostPropsTopicStatusBy), msgAndArgs...)
	}

	t.Run("on update, patch and the plugin API's update", func(t *testing.T) {
		edit := stored.Clone()
		edit.Message = "Edited"
		edit.SetProps(maps.Clone(forged))
		updated, _, appErr := th.App.UpdatePost(th.Context, edit, nil)
		require.Nil(t, appErr)
		requireKept(t, updated, "update")

		updated, _, appErr = th.App.PatchPost(th.Context, f.topic.Id, &model.PostPatch{Props: &forged}, nil)
		require.Nil(t, appErr)
		requireKept(t, updated, "patch")

		empty := model.StringInterface{}
		updated, _, appErr = th.App.PatchPost(th.Context, f.topic.Id, &model.PostPatch{Props: &empty}, nil)
		require.Nil(t, appErr)
		requireKept(t, updated, "a patch without them doesn't erase them")

		api := NewPluginAPI(th.App, th.Context, &model.Manifest{Id: "some.app"})
		appEdit := stored.Clone()
		appEdit.SetProps(maps.Clone(forged))
		updated, appErr = api.UpdatePost(appEdit)
		require.Nil(t, appErr)
		requireKept(t, updated, "an app")

		final, err := th.App.Srv().Store().Post().GetSingle(th.Context, f.topic.Id, false)
		require.NoError(t, err)
		requireKept(t, final, "stored")
	})
}

func TestMoveThreadIntoForum(t *testing.T) {
	mainHelper.Parallel(t)
	f := setupForum(t, model.ChannelPresetForumAgora)
	th := f.th
	user := f.member
	thread := func(t *testing.T, title string) *model.Post {
		t.Helper()
		root := &model.Post{UserId: user.Id, ChannelId: th.BasicChannel.Id, Message: "In a text channel"}
		if title != "" {
			root.AddProp(model.PostPropsThreadTitle, title)
		}
		root, _, appErr := th.App.CreatePost(th.Context, root, th.BasicChannel, model.CreatePostFlags{})
		require.Nil(t, appErr)
		_, _, appErr = th.App.CreatePost(th.Context, &model.Post{UserId: user.Id, ChannelId: th.BasicChannel.Id, RootId: root.Id, Message: "Its reply"}, th.BasicChannel, model.CreatePostFlags{})
		require.Nil(t, appErr)
		return root
	}

	t.Run("an untitled thread needs a title", func(t *testing.T) {
		root := thread(t, "")
		_, appErr := th.App.MoveThreadToChannel(th.Context, root.Id, f.forum.Id, false, user)
		requireBadRequest(t, appErr, "app.post.move_thread.forum_title_required.app_error")
		appErr = th.App.MoveThread(th.Context, root.Id, th.BasicChannel.Id, f.forum.Id, user)
		requireBadRequest(t, appErr, "app.post.move_thread.forum_title_required.app_error", "the other move")
		_, appErr = th.App.GetSinglePost(th.Context, root.Id, false)
		require.Nil(t, appErr, "the thread stayed where it was")

		moved, appErr := th.App.MoveThreadToChannelWithTitle(th.Context, root.Id, f.forum.Id, false, " Moved in ", user)
		require.Nil(t, appErr)
		assert.Equal(t, "Moved in", moved.GetProp(model.PostPropsThreadTitle))
		assert.Equal(t, f.forum.Id, moved.ChannelId)
	})

	t.Run("a named thread keeps its name", func(t *testing.T) {
		root := thread(t, "Named already")
		moved, appErr := th.App.MoveThreadToChannel(th.Context, root.Id, f.forum.Id, false, user)
		require.Nil(t, appErr)
		assert.Equal(t, "Named already", moved.GetProp(model.PostPropsThreadTitle))
	})

	t.Run("it can't be flattened into a forum", func(t *testing.T) {
		root := thread(t, "Named")
		_, appErr := th.App.MoveThreadToChannelWithTitle(th.Context, root.Id, f.forum.Id, true, "Title", user)
		requireBadRequest(t, appErr, "app.post.move_thread.forum_flatten.app_error")
	})

	t.Run("a title too long", func(t *testing.T) {
		root := thread(t, "")
		_, appErr := th.App.MoveThreadToChannelWithTitle(th.Context, root.Id, f.forum.Id, false, strings.Repeat("t", model.ForumTopicTitleMaxRunes+1), user)
		requireBadRequest(t, appErr, "app.post.forum.title_too_long.app_error")
	})
}

func TestImportForumPosts(t *testing.T) {
	mainHelper.Parallel(t)
	f := setupForum(t, model.ChannelPresetForumAgora)
	th := f.th
	line := func(props *model.StringInterface) imports.LineImportWorkerData {
		return imports.LineImportWorkerData{
			LineImportData: imports.LineImportData{Post: &imports.PostImportData{
				Team:     &th.BasicTeam.Name,
				Channel:  &f.forum.Name,
				User:     &th.BasicUser.Username,
				Message:  model.NewPointer("Imported " + model.NewId()),
				CreateAt: model.NewPointer(model.GetMillis()),
				Props:    props,
			}},
			LineNumber: 7,
		}
	}

	errLine, appErr := th.App.importMultiplePostLines(th.Context, []imports.LineImportWorkerData{line(nil)}, false, true, false, &imports.ImportReport{})
	requireBadRequest(t, appErr, "app.import.validate_post_import_data.forum_title_required.error")
	assert.Equal(t, 7, errLine)

	errLine, appErr = th.App.importMultiplePostLines(th.Context, []imports.LineImportWorkerData{line(&model.StringInterface{model.PostPropsThreadTitle: "Imported topic"})}, false, true, false, &imports.ImportReport{})
	require.Nil(t, appErr)
	assert.Equal(t, 0, errLine)
}
