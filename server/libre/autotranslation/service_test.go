// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package autotranslation

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/mattermost/mattermost/server/public/model"
	"github.com/mattermost/mattermost/server/public/shared/request"
	"github.com/mattermost/mattermost/server/v8/channels/app"
	"github.com/mattermost/mattermost/server/v8/channels/store"
)

const englishMessage = "Good morning everyone, the release build finished and all the tests are passing now."

func setupChannel(env *testEnv, channelType model.ChannelType, enabled bool) *model.Channel {
	ch := &model.Channel{Id: model.NewId(), Type: channelType, AutoTranslation: enabled}
	env.chStore.On("Get", ch.Id, true).Return(ch, nil)
	return ch
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	require.Eventually(t, cond, 5*time.Second, 10*time.Millisecond)
}

func TestFeatureAvailability(t *testing.T) {
	cfg := newTestConfig(providerLibreTranslate, "http://lt.example.com")
	ok, _ := featureAvailable(cfg)
	assert.True(t, ok)

	cfg.FeatureFlags.AutoTranslation = false
	ok, _ = featureAvailable(cfg)
	assert.False(t, ok)

	cfg = newTestConfig(providerLibreTranslate, "")
	ok, _ = featureAvailable(cfg)
	assert.False(t, ok)

	cfg = newTestConfig("deepl", "http://lt.example.com")
	ok, _ = featureAvailable(cfg)
	assert.False(t, ok)

	cfg = newTestConfig(providerAgents, "")
	ok, _ = featureAvailable(cfg)
	assert.True(t, ok)

	cfg.AutoTranslationSettings.Enable = model.NewPointer(false)
	ok, _ = featureAvailable(cfg)
	assert.False(t, ok)
}

func TestUnavailableErrors(t *testing.T) {
	env := newTestEnv(t, providerLibreTranslate, "http://lt.example.com")
	cfg := newTestConfig(providerLibreTranslate, "http://lt.example.com")
	cfg.AutoTranslationSettings.Enable = model.NewPointer(false)
	env.h.setConfig(cfg)

	assert.False(t, env.svc.IsFeatureAvailable())

	enabled, appErr := env.svc.IsChannelEnabled(model.NewId())
	assert.Nil(t, appErr)
	assert.False(t, enabled)

	_, appErr = env.svc.Translate(context.Background(), model.TranslationObjectTypePost, model.NewId(), model.NewId(), model.NewId(), "hello")
	require.NotNil(t, appErr)
	var notAvail *model.ErrAutoTranslationNotAvailable
	assert.True(t, errors.As(appErr, &notAvail))

	_, appErr = env.svc.GetBatch(model.TranslationObjectTypePost, []string{model.NewId()}, "en")
	require.NotNil(t, appErr)
	assert.True(t, errors.As(appErr, &notAvail))

	_, appErr = env.svc.GetUserLanguage(model.NewId(), model.NewId())
	require.NotNil(t, appErr)

	_, _, appErr = env.svc.DetectRemote(context.Background(), "hello")
	require.NotNil(t, appErr)
}

func TestChannelAndUserEnablement(t *testing.T) {
	env := newTestEnv(t, providerLibreTranslate, "http://lt.example.com")
	open := setupChannel(env, model.ChannelTypeOpen, true)
	off := setupChannel(env, model.ChannelTypeOpen, false)
	dm := setupChannel(env, model.ChannelTypeDirect, true)
	missing := model.NewId()
	env.chStore.On("Get", missing, true).Return(nil, store.NewErrNotFound("Channel", missing))

	check := func(id string, want bool) {
		t.Helper()
		got, appErr := env.svc.IsChannelEnabled(id)
		require.Nil(t, appErr)
		assert.Equal(t, want, got)
	}
	check(open.Id, true)
	check(off.Id, false)
	check(dm.Id, true)
	check(missing, false)

	cfg := newTestConfig(providerLibreTranslate, "http://lt.example.com")
	cfg.AutoTranslationSettings.RestrictDMAndGM = model.NewPointer(true)
	env.h.setConfig(cfg)
	check(dm.Id, false)
	check(open.Id, true)

	_, appErr := env.svc.IsChannelEnabled("bad")
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.autotranslation.validate_id.invalid", appErr.Id)

	fr, pt, other := model.NewId(), model.NewId(), model.NewId()
	env.at.addMember(open.Id, fr, "fr")
	env.at.addMember(open.Id, pt, "pt-BR")

	enabled, appErr := env.svc.IsUserEnabled(open.Id, pt)
	require.Nil(t, appErr)
	assert.True(t, enabled)
	enabled, appErr = env.svc.IsUserEnabled(open.Id, other)
	require.Nil(t, appErr)
	assert.False(t, enabled)

	lang, appErr := env.svc.GetUserLanguage(pt, open.Id)
	require.Nil(t, appErr)
	assert.Equal(t, "pt", lang)

	lang, appErr = env.svc.GetUserLanguage(fr, open.Id)
	require.Nil(t, appErr)
	assert.Equal(t, "", lang, "fr is not a target language")
}

func TestTranslatePostEndToEnd(t *testing.T) {
	lt := newFakeLibreTranslate(t, "")
	env := newTestEnv(t, providerLibreTranslate, lt.URL)
	require.NoError(t, env.svc.Start())

	ch := setupChannel(env, model.ChannelTypeOpen, true)
	author, reader, reader2, french := model.NewId(), model.NewId(), model.NewId(), model.NewId()
	env.at.addMember(ch.Id, author, "en")
	env.at.addMember(ch.Id, reader, "es")
	env.at.addMember(ch.Id, reader2, "pt-BR")
	env.at.addMember(ch.Id, french, "fr")

	post := &model.Post{Id: model.NewId(), ChannelId: ch.Id, UserId: author, Message: englishMessage + " cc @reader"}
	tr, appErr := env.svc.Translate(context.Background(), model.TranslationObjectTypePost, post.Id, ch.Id, author, post)
	require.Nil(t, appErr)

	// The author reads English, the language of the post.
	require.NotNil(t, tr)
	assert.Equal(t, "en", tr.Lang)
	assert.Equal(t, model.TranslationStateSkipped, tr.State)

	// Every target language used in the channel is attached to the post
	// broadcast; French is not a target language.
	require.NotNil(t, post.Metadata)
	require.Len(t, post.Metadata.Translations, 3)
	assert.Equal(t, "skipped", post.Metadata.Translations["en"].State)
	assert.Equal(t, "en", post.Metadata.Translations["en"].SourceLang)
	assert.Contains(t, []string{"processing", "ready"}, post.Metadata.Translations["es"].State)

	waitFor(t, func() bool {
		es := env.at.get(model.TranslationObjectTypePost, post.Id, "es")
		pt := env.at.get(model.TranslationObjectTypePost, post.Id, "pt")
		return es != nil && es.State == model.TranslationStateReady && pt != nil && pt.State == model.TranslationStateReady
	})

	es := env.at.get(model.TranslationObjectTypePost, post.Id, "es")
	assert.Equal(t, model.TranslationTypeObject, es.Type)
	assert.Equal(t, providerLibreTranslate, es.Provider)
	assert.Equal(t, "en", es.Meta[metaSrcLang])
	var obj map[string]string
	require.NoError(t, json.Unmarshal(es.ObjectJSON, &obj))
	assert.Equal(t, "es:"+englishMessage+" cc @reader", obj["message"], "mentions are restored")
	assert.Equal(t, ch.Id, es.ChannelID)

	// Source language came from local detection: no remote detect call, one
	// translate call per language.
	translateCalls, detectCalls := lt.counts()
	assert.Equal(t, 2, translateCalls)
	assert.Equal(t, 0, detectCalls)
	assert.Equal(t, "en", lt.lastRequest()["source"])

	// Clients are notified with the base language and the locales behind it.
	waitFor(t, func() bool { return len(env.h.publishedEvents()) == 2 })
	var ptEvent *model.WebSocketEvent
	for _, ev := range env.h.publishedEvents() {
		assert.Equal(t, model.WebsocketEventPostTranslationUpdated, ev.EventType())
		assert.Equal(t, ch.Id, ev.GetBroadcast().ChannelId)
		assert.Equal(t, post.Id, ev.GetData()["object_id"])
		if _, ok := ev.GetData()["translations"].(map[string]any)["pt-BR"]; ok {
			ptEvent = ev
		}
	}
	require.NotNil(t, ptEvent)
	entry := ptEvent.GetData()["translations"].(map[string]any)["pt"].(map[string]any)
	assert.Equal(t, "ready", entry["state"])
	assert.Equal(t, "en", entry["src_lang"])
	assert.JSONEq(t, `{"message":"pt:`+englishMessage+` cc @reader"}`, entry["translation"].(string))

	// A reader gets their translation back.
	tr, appErr = env.svc.Translate(context.Background(), model.TranslationObjectTypePost, post.Id, ch.Id, reader, post)
	require.Nil(t, appErr)
	require.NotNil(t, tr)
	assert.Equal(t, model.TranslationStateReady, tr.State)
	translateCalls, _ = lt.counts()
	assert.Equal(t, 2, translateCalls, "unchanged content reuses stored translations")

	// GetBatch serves the stored translations.
	batch, appErr := env.svc.GetBatch(model.TranslationObjectTypePost, []string{post.Id}, "es-ES")
	require.Nil(t, appErr)
	require.Contains(t, batch, post.Id)
	assert.Equal(t, model.TranslationStateReady, batch[post.Id].State)

	// Editing the post re-translates it.
	post.Message = "The release is postponed until tomorrow because a blocking regression was found."
	post.Metadata = nil
	_, appErr = env.svc.Translate(context.Background(), model.TranslationObjectTypePost, post.Id, ch.Id, author, post)
	require.Nil(t, appErr)
	waitFor(t, func() bool {
		es := env.at.get(model.TranslationObjectTypePost, post.Id, "es")
		return es != nil && es.State == model.TranslationStateReady && string(es.ObjectJSON) == `{"message":"es:`+post.Message+`"}`
	})
}

func TestTranslateRemoteDetection(t *testing.T) {
	lt := newFakeLibreTranslate(t, "")
	env := newTestEnv(t, providerLibreTranslate, lt.URL)
	require.NoError(t, env.svc.Start())

	ch := setupChannel(env, model.ChannelTypeOpen, true)
	es, en := model.NewId(), model.NewId()
	env.at.addMember(ch.Id, es, "es")
	env.at.addMember(ch.Id, en, "en")

	// Too short to trust local detection: the provider detects it.
	post := &model.Post{Id: model.NewId(), ChannelId: ch.Id, UserId: es, Message: "Hola a todos"}
	tr, appErr := env.svc.Translate(context.Background(), model.TranslationObjectTypePost, post.Id, ch.Id, es, post)
	require.Nil(t, appErr)
	require.NotNil(t, tr)
	assert.Equal(t, model.TranslationStateProcessing, tr.State)

	waitFor(t, func() bool {
		esT := env.at.get(model.TranslationObjectTypePost, post.Id, "es")
		enT := env.at.get(model.TranslationObjectTypePost, post.Id, "en")
		return esT != nil && esT.State == model.TranslationStateSkipped && enT != nil && enT.State == model.TranslationStateReady
	})
	translateCalls, detectCalls := lt.counts()
	assert.Equal(t, 1, detectCalls)
	assert.Equal(t, 1, translateCalls, "the source language is not translated")
}

func TestTranslateValidation(t *testing.T) {
	env := newTestEnv(t, providerLibreTranslate, "http://lt.example.com")
	require.NoError(t, env.svc.Start())
	ch := setupChannel(env, model.ChannelTypeOpen, true)
	user := model.NewId()
	env.at.addMember(ch.Id, user, "es")

	_, appErr := env.svc.Translate(context.Background(), "", model.NewId(), ch.Id, user, "hi")
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.autotranslation.add_task.missing_object_type", appErr.Id)

	_, appErr = env.svc.Translate(context.Background(), model.TranslationObjectTypePost, "", ch.Id, user, "hi")
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.autotranslation.validate_id.empty", appErr.Id)

	post := &model.Post{Id: model.NewId(), Message: "https://example.com @someone"}
	_, appErr = env.svc.Translate(context.Background(), model.TranslationObjectTypePost, post.Id, ch.Id, user, post)
	require.NotNil(t, appErr)
	assert.Equal(t, "ent.autotranslation.no_translatable_content", appErr.Id)

	// A user who did not opt in gets nothing back, but the translations
	// for the channel are still initialized.
	outsider := model.NewId()
	post = &model.Post{Id: model.NewId(), Message: englishMessage}
	tr, appErr := env.svc.Translate(context.Background(), model.TranslationObjectTypePost, post.Id, ch.Id, outsider, post)
	require.Nil(t, appErr)
	assert.Nil(t, tr)
	assert.NotNil(t, env.at.get(model.TranslationObjectTypePost, post.Id, "es"))
}

func TestTranslateProviderFailure(t *testing.T) {
	lt := newFakeLibreTranslate(t, "")
	lt.failWith = 500
	env := newTestEnv(t, providerLibreTranslate, lt.URL)
	require.NoError(t, env.svc.Start())
	ch := setupChannel(env, model.ChannelTypeOpen, true)
	user := model.NewId()
	env.at.addMember(ch.Id, user, "es")

	post := &model.Post{Id: model.NewId(), Message: englishMessage}
	_, appErr := env.svc.Translate(context.Background(), model.TranslationObjectTypePost, post.Id, ch.Id, user, post)
	require.Nil(t, appErr)

	waitFor(t, func() bool {
		tr := env.at.get(model.TranslationObjectTypePost, post.Id, "es")
		return tr != nil && tr.State == model.TranslationStateUnavailable
	})
	tr := env.at.get(model.TranslationObjectTypePost, post.Id, "es")
	assert.Contains(t, tr.Meta[metaError], "500")
	waitFor(t, func() bool { return len(env.h.publishedEvents()) == 1 })
}

func TestTranslateWithAgents(t *testing.T) {
	env := newTestEnv(t, providerAgents, "")
	env.h.agentsAvailable = true
	env.h.agentsReply = func(serviceID string, req app.BridgeCompletionRequest) (string, error) {
		return `{"translations":["Hola {{0}}, la versión está lista para todos los equipos."],"source_language":"en","confidence":0.99}`, nil
	}
	require.NoError(t, env.svc.Start())
	ch := setupChannel(env, model.ChannelTypeOpen, true)
	user := model.NewId()
	env.at.addMember(ch.Id, user, "es")

	post := &model.Post{Id: model.NewId(), Message: "Hello @all, the release is ready for all the teams."}
	_, appErr := env.svc.Translate(context.Background(), model.TranslationObjectTypePost, post.Id, ch.Id, user, post)
	require.Nil(t, appErr)

	waitFor(t, func() bool {
		tr := env.at.get(model.TranslationObjectTypePost, post.Id, "es")
		return tr != nil && tr.State == model.TranslationStateReady
	})
	tr := env.at.get(model.TranslationObjectTypePost, post.Id, "es")
	assert.JSONEq(t, `{"message":"Hola @all, la versión está lista para todos los equipos."}`, string(tr.ObjectJSON))
	assert.Equal(t, providerAgents, tr.Provider)
}

func TestRecoverStuck(t *testing.T) {
	lt := newFakeLibreTranslate(t, "")
	env := newTestEnv(t, providerLibreTranslate, lt.URL)
	require.NoError(t, env.svc.Start())
	ch := setupChannel(env, model.ChannelTypeOpen, true)
	user := model.NewId()
	env.at.addMember(ch.Id, user, "es")

	post := &model.Post{Id: model.NewId(), ChannelId: ch.Id, Message: englishMessage}
	content, appErr := extractContent(post)
	require.Nil(t, appErr)
	env.pStore.On("GetSingle", mock.Anything, post.Id, false).Return(post, nil)

	gone := model.NewId()
	env.pStore.On("GetSingle", mock.Anything, gone, false).Return(nil, store.NewErrNotFound("Post", gone))

	now := model.GetMillis()
	// Stuck for ten minutes: retried.
	env.at.put(&model.Translation{ObjectID: post.Id, ObjectType: model.TranslationObjectTypePost, Lang: "es", Type: model.TranslationTypeObject,
		State: model.TranslationStateProcessing, NormHash: content.NormHash, UpdateAt: now - 10*60*1000})
	// Stuck for two hours: given up.
	env.at.put(&model.Translation{ObjectID: post.Id, ObjectType: model.TranslationObjectTypePost, Lang: "pt", Type: model.TranslationTypeObject,
		State: model.TranslationStateProcessing, NormHash: content.NormHash, UpdateAt: now - 2*60*60*1000})
	// Post deleted: marked unavailable.
	env.at.put(&model.Translation{ObjectID: gone, ObjectType: model.TranslationObjectTypePost, Lang: "es", Type: model.TranslationTypeObject,
		State: model.TranslationStateProcessing, NormHash: "x", UpdateAt: now - 10*60*1000})
	// Recent: left alone.
	recent := model.NewId()
	env.at.put(&model.Translation{ObjectID: recent, ObjectType: model.TranslationObjectTypePost, Lang: "es", Type: model.TranslationTypeObject,
		State: model.TranslationStateProcessing, NormHash: "y", UpdateAt: now})

	found, err := env.svc.recoverStuck(request.EmptyContext(env.h.logger))
	require.NoError(t, err)
	assert.Equal(t, 3, found)

	waitFor(t, func() bool {
		tr := env.at.get(model.TranslationObjectTypePost, post.Id, "es")
		return tr != nil && tr.State == model.TranslationStateReady
	})
	assert.Equal(t, model.TranslationStateUnavailable, env.at.get(model.TranslationObjectTypePost, post.Id, "pt").State)
	assert.Equal(t, model.TranslationStateUnavailable, env.at.get(model.TranslationObjectTypePost, gone, "es").State)
	assert.Equal(t, model.TranslationStateProcessing, env.at.get(model.TranslationObjectTypePost, recent, "es").State)
	assert.Equal(t, ch.Id, env.at.get(model.TranslationObjectTypePost, post.Id, "es").ChannelID)
}

func TestWorkerPoolDedupe(t *testing.T) {
	release := make(chan struct{})
	processed := make(chan *task, 10)
	var pool *workerPool
	pool = newWorkerPool(func(tk *task) {
		<-release
		for _, l := range tk.langs {
			pool.done(tk, l)
		}
		processed <- tk
	}, nil)

	content := &sourceContent{NormHash: "h"}
	require.ErrorIs(t, pool.enqueue(&task{objectType: "post", objectID: "a", content: content, langs: []string{"es"}}), errWorkerStopped)

	pool.start(1)
	defer pool.stop()

	require.NoError(t, pool.enqueue(&task{objectType: "post", objectID: "a", content: content, langs: []string{"es", "de"}}))
	second := &task{objectType: "post", objectID: "a", content: content, langs: []string{"es", "pt"}}
	require.NoError(t, pool.enqueue(second))
	assert.Equal(t, []string{"pt"}, second.langs, "languages already queued are dropped")
	require.NoError(t, pool.enqueue(&task{objectType: "post", objectID: "a", content: content, langs: []string{"de"}}))
	assert.True(t, pool.isInflight("post", "a"))
	assert.False(t, pool.isInflight("post", "b"))

	close(release)
	<-processed
	<-processed
	waitFor(t, func() bool { return !pool.isInflight("post", "a") })
}
