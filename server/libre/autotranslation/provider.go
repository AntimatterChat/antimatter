// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package autotranslation

import (
	"context"
	"errors"
	"time"

	"github.com/mattermost/mattermost/server/public/model"
)

const (
	providerLibreTranslate = "libretranslate"
	providerAgents         = "agents"
)

var (
	errAgentsUnavailable = errors.New("the Agents plugin bridge is not available")
	errNoProvider        = errors.New("no translation provider is configured")
)

// translateResult is the output of a provider translation call.
type translateResult struct {
	// Texts holds one translated string per input segment, in order.
	Texts []string
	// SourceLang is the source language the provider detected, when the
	// caller did not supply one. May be empty.
	SourceLang string
	// Confidence is the detection confidence in [0, 1], when known.
	Confidence *float64
}

// provider is a translation backend.
type provider interface {
	// Name identifies the provider; it is stored with each translation.
	Name() string
	// Translate translates texts into dst. src is the source language, or ""
	// to let the provider detect it.
	Translate(ctx context.Context, texts []string, src, dst string) (*translateResult, error)
	// Detect returns the language of text and the confidence in [0, 1].
	Detect(ctx context.Context, text string) (string, *float64, error)
}

// newProvider builds the provider selected in the configuration. It returns
// errNoProvider when the configuration does not name a usable provider.
func newProvider(h host, cfg *model.Config) (provider, error) {
	settings := cfg.AutoTranslationSettings
	timeout := providerTimeout(cfg)
	switch model.SafeDereference(settings.Provider) {
	case providerLibreTranslate:
		if settings.LibreTranslate == nil || model.SafeDereference(settings.LibreTranslate.URL) == "" {
			return nil, errNoProvider
		}
		return newLibreTranslateProvider(
			h.HTTPClient(),
			model.SafeDereference(settings.LibreTranslate.URL),
			model.SafeDereference(settings.LibreTranslate.APIKey),
			timeout,
		), nil
	case providerAgents:
		if settings.Agents == nil || model.SafeDereference(settings.Agents.LLMServiceID) == "" {
			return nil, errNoProvider
		}
		return newAgentsProvider(h, model.SafeDereference(settings.Agents.LLMServiceID)), nil
	default:
		return nil, errNoProvider
	}
}

// providerTimeout returns the per-call timeout configured for the provider.
func providerTimeout(cfg *model.Config) time.Duration {
	ms := model.SafeDereference(cfg.AutoTranslationSettings.TimeoutMs)
	if ms <= 0 {
		ms = 5000
	}
	return time.Duration(ms) * time.Millisecond
}
