// Copyright (c) 2026-present Mattermost Libre contributors.
// See LICENSE.txt for license information.

package autotranslation

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"sort"
	"strings"

	"github.com/mattermost/mattermost/server/public/model"
)

const (
	// maxContentBytes caps the total size of the text of an object sent for
	// translation. It matches the largest post message the server accepts.
	maxContentBytes = model.PostMessageMaxBytesV2
	// maxObjectDepth bounds the recursion into JSON objects.
	maxObjectDepth = 16
)

// sourceContent is the translatable view of an object: an ordered list of
// masked text segments plus what is needed to rebuild the translated value.
type sourceContent struct {
	Type model.TranslationType
	// Segments are the masked strings to translate.
	Segments []maskedText
	// Paths locate each segment inside Object for TranslationTypeObject
	// content. For posts the only path is ["message"].
	Paths [][]string
	// Object is the decoded original object for TranslationTypeObject.
	Object map[string]any
	// NormHash identifies the normalized source text.
	NormHash string
	// Plain is the unmasked, concatenated text, used for language detection.
	Plain string
}

// segmentTexts returns the masked texts to hand to the provider.
func (c *sourceContent) segmentTexts() []string {
	texts := make([]string, len(c.Segments))
	for i, s := range c.Segments {
		texts[i] = s.Text
	}
	return texts
}

// build reassembles the translated value from the provider output. It
// returns the text for string content, or the JSON document for object
// content.
func (c *sourceContent) build(translated []string) (string, json.RawMessage, error) {
	restored := make([]string, len(translated))
	for i, t := range translated {
		r, err := c.Segments[i].unmask(t)
		if err != nil {
			return "", nil, err
		}
		restored[i] = r
	}

	if c.Type == model.TranslationTypeString {
		return restored[0], nil, nil
	}

	obj := deepCopyMap(c.Object)
	for i, path := range c.Paths {
		setPath(obj, path, restored[i])
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return "", nil, err
	}
	return "", data, nil
}

func contentError(id, details string, status int) *model.AppError {
	return model.NewAppError("AutoTranslation.Translate", id, nil, details, status)
}

// extractContent turns the value handed to Translate into a sourceContent.
// Supported values are a string, a *model.Post, a json.RawMessage holding an
// object, and a map[string]any.
func extractContent(content any) (*sourceContent, *model.AppError) {
	var c *sourceContent
	switch v := content.(type) {
	case nil:
		return nil, contentError("ent.autotranslation.validate_content.nil_content", "", http.StatusBadRequest)
	case *model.Post:
		if v == nil {
			return nil, contentError("ent.autotranslation.extract_post.nil_post", "", http.StatusBadRequest)
		}
		// Only regular user messages are rendered translated by clients;
		// system and plugin-typed posts carry generated text.
		if v.Type != model.PostTypeDefault {
			return nil, contentError("ent.autotranslation.no_translatable_content", "post type "+v.Type, http.StatusBadRequest)
		}
		c = &sourceContent{
			Type:   model.TranslationTypeObject,
			Object: map[string]any{"message": v.Message},
			Paths:  [][]string{{"message"}},
		}
		c.Segments = []maskedText{maskText(v.Message)}
	case string:
		c = &sourceContent{Type: model.TranslationTypeString, Segments: []maskedText{maskText(v)}}
	case json.RawMessage:
		var obj map[string]any
		if err := json.Unmarshal(v, &obj); err != nil || obj == nil {
			details := "content is not a JSON object"
			if err != nil {
				details = err.Error()
			}
			return nil, contentError("ent.autotranslation.validate_content.invalid_json", details, http.StatusBadRequest)
		}
		c = objectContent(obj)
	case []byte:
		return extractContent(json.RawMessage(v))
	case map[string]any:
		if v == nil {
			return nil, contentError("ent.autotranslation.validate_content.nil_content", "", http.StatusBadRequest)
		}
		// Round-trip through JSON so the stored copy holds only JSON types.
		data, err := json.Marshal(v)
		if err != nil {
			return nil, contentError("ent.autotranslation.validate_content.invalid_json", err.Error(), http.StatusBadRequest)
		}
		return extractContent(json.RawMessage(data))
	default:
		return nil, contentError("ent.autotranslation.validate_content.invalid_type", "", http.StatusBadRequest)
	}

	size := 0
	plain := make([]string, 0, len(c.Segments))
	anyTranslatable := false
	for _, s := range c.Segments {
		size += len(s.Text)
		if s.translatable() {
			anyTranslatable = true
		}
		plain = append(plain, s.plain())
	}
	if size > maxContentBytes {
		return nil, contentError("ent.autotranslation.validate_content.text_too_large", "", http.StatusRequestEntityTooLarge)
	}
	if !anyTranslatable {
		return nil, contentError("ent.autotranslation.no_translatable_content", "", http.StatusBadRequest)
	}
	c.Plain = strings.Join(plain, "\n")
	c.NormHash = normHash(c)
	return c, nil
}

// plain returns the translatable prose of a segment with the protected spans
// removed, for language detection.
func (m maskedText) plain() string {
	return strings.Join(strings.Fields(maskTokenRe.ReplaceAllString(m.Text, " ")), " ")
}

// normHash hashes the normalized source so unchanged content (for example a
// post edit touching only its props) reuses existing translations.
func normHash(c *sourceContent) string {
	h := sha256.New()
	h.Write([]byte(c.Type))
	for i, s := range c.Segments {
		h.Write([]byte{0})
		if c.Paths != nil {
			h.Write([]byte(strings.Join(c.Paths[i], "\x1f")))
			h.Write([]byte{0})
		}
		h.Write([]byte(normalizeWhitespace(restoreForHash(s))))
	}
	return hex.EncodeToString(h.Sum(nil))
}

func restoreForHash(s maskedText) string {
	r, err := s.unmask(s.Text)
	if err != nil {
		return s.Text
	}
	return r
}

func normalizeWhitespace(s string) string {
	lines := strings.Split(strings.ReplaceAll(s, "\r\n", "\n"), "\n")
	for i, l := range lines {
		lines[i] = strings.TrimRight(l, " \t")
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

// objectContent collects the translatable string leaves of a JSON object.
func objectContent(obj map[string]any) *sourceContent {
	c := &sourceContent{Type: model.TranslationTypeObject, Object: obj}
	collectStrings(obj, nil, 0, c)
	return c
}

func collectStrings(obj map[string]any, prefix []string, depth int, c *sourceContent) {
	if depth > maxObjectDepth {
		return
	}
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, k := range keys {
		if skipKey(k) {
			continue
		}
		path := append(append([]string(nil), prefix...), k)
		switch v := obj[k].(type) {
		case string:
			if model.IsValidId(v) {
				continue
			}
			m := maskText(v)
			if !m.translatable() {
				continue
			}
			c.Paths = append(c.Paths, path)
			c.Segments = append(c.Segments, m)
		case map[string]any:
			collectStrings(v, path, depth+1, c)
		}
	}
}

// skipKey reports whether a JSON key names an identifier or machine value
// rather than prose.
func skipKey(k string) bool {
	lk := strings.ToLower(k)
	switch lk {
	case "id", "type", "url", "uri", "href", "src", "link", "email", "username", "channel", "team", "icon", "color", "props", "key":
		return true
	}
	return strings.HasSuffix(lk, "_id") || strings.HasSuffix(k, "Id") || strings.HasSuffix(k, "ID") ||
		strings.HasSuffix(lk, "_url") || strings.HasSuffix(k, "Url") || strings.HasSuffix(k, "URL") ||
		strings.HasSuffix(lk, "_at") || strings.HasSuffix(k, "At")
}

func deepCopyMap(m map[string]any) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if sub, ok := v.(map[string]any); ok {
			out[k] = deepCopyMap(sub)
		} else {
			out[k] = v
		}
	}
	return out
}

func setPath(obj map[string]any, path []string, value string) {
	cur := obj
	for _, k := range path[:len(path)-1] {
		next, ok := cur[k].(map[string]any)
		if !ok {
			return
		}
		cur = next
	}
	cur[path[len(path)-1]] = value
}
