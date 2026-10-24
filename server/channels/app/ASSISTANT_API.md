# Assistant API (stage 1)

The server side of the ambient assistant (`AI_ASSISTANT_DESIGN.md`, decisions of 2026-10-24), for the panel app
(antimatter-plugin-assistant) and mobile (`app/products/assistant`). The routes and events are the server's own:
they don't depend on any app. Code: `assistant.go` (feeds, presence, settings), `assistant_run.go` (windows),
`assistant_rules.go` and `assistant_model.go` (engines), `assistant_offer.go` (actions' offers),
`../api4/assistant.go` (routes). Client: `Client4` in `webapp/platform/client`, types in
`webapp/platform/types/src/assistant.ts`.

Every route needs a session and is the session's person's own. Nobody can read or change another person's feed.
A card ID of someone else's is "not found" (404). Bots and deactivated accounts get 403.

## How a panel uses it

1. When the panel shows a channel: `POST /presence` with `{"open": true}`, then again every `heartbeat_seconds`
   (30) while it stays open. Presence lapses `ttl_seconds` (90) after the last call. When the panel closes or
   moves to another channel: `{"open": false}`.
2. `GET /feed` for the cards, then the websocket events keep it current. Show `following` (presence) as the
   "assistant is following this conversation" indicator.
3. Card controls: Post to channel (insights), Pin, Dismiss, "Don't suggest this kind again" (mute), and for actions
   Run (only when `offer.can_run`), otherwise "Offered to <holder_id's name>". Claim is for an action offered to
   nobody (`holder_id` ""), which happens only when nobody who may run it was viewing.
4. Nothing notifies: no push, no badge from the server. The panel's own unread count comes from
   `assistant_card_added`.

Base path: `/api/v4/assistant`.

## Routes

### GET /channels/{channel_id}/feed

The person's feed in a channel they may read (a member, for private channels, direct and group messages).
Pinned cards come first, then the newest. Dismissed cards aren't listed.

```json
{
  "channel_id": "4xp9fdq7xjgd3e1b8a3k5w6m1c",
  "enabled": true,
  "on": true,
  "engine": "rules",
  "cards": [
    {
      "id": "w7c3b8k1t3fz8q5n6r1e9y2u4a",
      "user_id": "9a3kd8w1p4bq7fz3m6n2x5c8ho",
      "channel_id": "4xp9fdq7xjgd3e1b8a3k5w6m1c",
      "kind": "insight",
      "tool": "mcp.grafana.query_prometheus",
      "tool_title": "Query Prometheus",
      "arguments": {"host": "www-1", "metric": "cpu"},
      "why": {
        "engine": "rules",
        "pattern": "ops.resource_usage",
        "words": ["cpu on www-1"],
        "entities": [{"type": "host", "value": "www-1", "text": "www-1"}]
      },
      "post_ids": ["p1q7x3m9d4hf8bk2e5r6w1t0ny"],
      "dedupe_key": "6f1c2b0e9a8d7c6b5a4f3e2d1c0b9a8f",
      "result": "www-1 CPU, last hour: 23% average, 61% peak at 14:05",
      "status": "new",
      "pinned": false,
      "create_at": 1792850400000,
      "update_at": 1792850400000
    },
    {
      "id": "k2m8x4c1b7nz3q9f5r6t1y0u2a",
      "user_id": "9a3kd8w1p4bq7fz3m6n2x5c8ho",
      "channel_id": "4xp9fdq7xjgd3e1b8a3k5w6m1c",
      "kind": "action",
      "tool": "mcp.ci.deploy",
      "tool_title": "Deploy",
      "arguments": {"service": "web-app"},
      "why": {
        "engine": "model",
        "probabilities": {"card": 0.91, "tool": 0.88, "service": 0.95}
      },
      "post_ids": ["r5t1y8u2i9op3as4df6gh7jk0l", "z9x8c7v6b5nm4as3df2gh1jk0q"],
      "dedupe_key": "0e9d8c7b6a5f4e3d2c1b0a9f8e7d6c5b",
      "status": "new",
      "pinned": false,
      "offer_id": "o3f7h1k9m2q8x4c6v5b1n0z7wa",
      "offer": {
        "id": "o3f7h1k9m2q8x4c6v5b1n0z7wa",
        "status": "open",
        "holder_id": "5n8b2v6c1x9z4m7q3w0e8r1t6y",
        "offered_to_me": false,
        "can_run": false,
        "moves_at": 1792850520000
      },
      "create_at": 1792850410000,
      "update_at": 1792850410000
    }
  ]
}
```

- `enabled`: the assistant works in this channel (server on, community not off, channel not archived). `on`: the
  person's own switch for the channel.
- `kind`: `insight` (a tool that only reads; `result` is what it gave, as the person, at most 4000 characters) or
  `action` (a tool that changes something; never run on its own, no `result`).
- `why`: the rules engine names the `pattern`, the `words` that matched and the `entities`; the model engine gives
  `probabilities` (`card`, `tool`, and one per argument by its name).
  A card from a trusted app's post (below) also has `app`, the app's ID: show it as "From a post of <app>".
- `post_ids`: the triggering message first, then the rest of the window (model engine).
- `status`: `new`, `posted` (posted to the channel), `dismissed` (only in events).
- `offer` (actions): `status` `open`, `done` or `expired` (after a day); `holder_id` the person it's offered to now
  ("" for nobody); `offered_to_me`/`can_run` for the viewer; `moves_at` when it goes to the next person if the
  holder ignores it (0 when it doesn't); `done_by`/`done_at` once run.

### POST /channels/{channel_id}/presence

Request: `{"open": true}` (or `false`). Response:

```json
{"following": true, "ttl_seconds": 90, "heartbeat_seconds": 30}
```

`following` is false when the assistant is off there (server, community, the person's switch) or the channel is
archived; the presence isn't kept then.

### PUT /channels/{channel_id}/enabled

The person's switch for the channel (a preference: category `assistant`, name the channel's ID, value `on`/`off`).
Request and response: `{"on": false}`. Turning it off also closes the presence and forgets the feed there.

### POST /channels/{channel_id}/cards/{card_id}/dismiss

No body. Response: the card (`"status": "dismissed"`, with `offer` for an action). Dismissing an action passes it:
if it was offered to the person, it goes to the next; they never get it back.

### POST /channels/{channel_id}/cards/{card_id}/pin

Request: `{"pinned": true}`. Response: the card.

### POST /channels/{channel_id}/cards/{card_id}/mute

"Don't suggest this kind again": the card's tool is muted for the person (everywhere) and the card dismissed. No
body. Response: the card.

### POST /channels/{channel_id}/cards/{card_id}/claim

For an action offered to nobody: the person takes it if they may (they got the card, haven't passed it, and are
viewing). No body. Response: the card with its offer (`holder_id` the person). Already theirs: 200 unchanged.
Someone else's: 403 `app.assistant.offer_not_yours.app_error`. Done: 409 `app.assistant.offer_done.app_error`.

### POST /channels/{channel_id}/cards/{card_id}/run

Only the holder; once. No body. Response:

```json
{
  "card": {"id": "k2m8x4c1b7nz3q9f5r6t1y0u2a", "kind": "action", "offer": {"id": "o3f7h1k9m2q8x4c6v5b1n0z7wa", "status": "done", "holder_id": "9a3kd8w1p4bq7fz3m6n2x5c8ho", "offered_to_me": false, "can_run": false, "moves_at": 0, "done_by": "9a3kd8w1p4bq7fz3m6n2x5c8ho", "done_at": 1792850450000}, "...": "the rest of the card"},
  "executed": false
}
```

`executed` stays false in stage 1: the offer is claimed, closed for everyone and audited (`assistantRunAction`),
but the tool doesn't run yet (stage 3 runs it through the tool registry's confirmation). Not the holder: 403
`app.assistant.offer_not_yours.app_error`; a second run: 409 `app.assistant.offer_done.app_error`; the person lost
the tool: 403 `app.ai.tools.call_not_allowed.app_error`; assistant off or channel archived: 403
`app.assistant.off.app_error`; the offer expired: 410 `app.assistant.offer_gone.app_error`.

### POST /channels/{channel_id}/cards/{card_id}/post

Posts an insight's content to its channel as the person, with the normal permission check (`create_post`; not in
an archived channel). No body. Response: 201 and the post. Its message is `**<tool_title>**\n<result>`; its prop
`assistant_card` is the card's ID (the assistant ignores such messages). An action: 400
`app.assistant.post_insight_only.app_error`.

### GET /muted_tools, PUT /muted_tools

```json
{"tools": ["mcp.grafana.query_prometheus"]}
```

PUT replaces the list (at most 200 full tool names; a preference: category `assistant`, name `muted_tools`).

### GET /communities/{team_id}/settings, PUT /communities/{team_id}/settings

For the community's admins (`manage_team_ai_connections`, like its model connections and MCP servers). GET:

```json
{
  "team_id": "t8b2n6m1x4c9z7q3w5e0r8y1ua",
  "server_enabled": true,
  "may_override": true,
  "settings": {"enabled": null, "engine": "", "connection_id": "", "model": ""}
}
```

PUT takes `settings` (`{"enabled": false, "engine": "model", "connection_id": "<id>", "model": ""}`) and answers
the GET's shape. `enabled: null` and empty fields keep the server's; a community can turn the assistant off, not on
when the server has it off. The connection must be the community's own or a server-wide one shared with it (else
400 `app.assistant.community_connection.app_error`). The server must allow it (`AllowCommunityOverride`), else 403
`app.assistant.community_override.app_error`.

## Websocket events

All three go to one person's clients only (never a channel or a community), reliable across the cluster.

### assistant_card_added

A card in the person's feed. Data:

```json
{"channel_id": "4xp9fdq7xjgd3e1b8a3k5w6m1c", "card": {"id": "w7c3b8k1t3fz8q5n6r1e9y2u4a", "...": "the card, as in the feed"}}
```

### assistant_card_updated

The person dismissed, pinned, unpinned or posted it (from any of their clients). Same data as `card_added`.

### assistant_offer_updated

An action's offer moved to someone else, went to nobody, was claimed, or was run. Sent to everyone who has its
card, each with their own view. Data:

```json
{
  "channel_id": "4xp9fdq7xjgd3e1b8a3k5w6m1c",
  "card_id": "k2m8x4c1b7nz3q9f5r6t1y0u2a",
  "offer": {"id": "o3f7h1k9m2q8x4c6v5b1n0z7wa", "status": "open", "holder_id": "9a3kd8w1p4bq7fz3m6n2x5c8ho", "offered_to_me": true, "can_run": true, "moves_at": 1792850640000}
}
```

`card_id` is the recipient's own card of the offer.

## Settings (AssistantSettings)

A section of its own in the config (not under AISettings, which the Built-in features page saves whole). All under
`integrations_integration_management`.

| Setting | Default | |
|---|---|---|
| `Enable` | false | The server-wide switch. |
| `Engine` | `rules` | `rules` or `model`. |
| `AllowCommunityOverride` | false | Communities' admins may change it for their community. |
| `ConnectionId`, `Model` | "" | The model engine's decision model: a connection's decisions model (never its chat model). |
| `ThresholdPercent` | 70 | The probability the model engine needs (card, tool, each argument). |
| `DecisionsPerUserHour`, `DecisionsPerChannelHour` | 30, 60 | The model engine's hourly budgets (each decision call counts). |
| `CardsPerUserHour` | 20 | Cards per person per hour, both engines. |
| `DebounceSeconds` | 4 | Quiet in a channel before its window runs (at most 3× after its first message). |
| `OfferTimeoutSeconds` | 120 | How long an action waits for its holder before it moves on. |
| `AllowedTools` | `["*"]` | The tools the assistant may use, as agents' policies name them. |
| `Patterns` | built in | JSON list of `{id, kind, match[], tools[], arguments{}}`; replaces the built-in ones. |
| `SeedEntities` | none | JSON list of `{type, value, aliases[]}`. |
| `DictionarySources` | mcp-grafana's | JSON list of `{tool, type, field, arguments}`: read-only listings that fill the dictionary. |
| `TriggeringApps` | `["com.antimatterchat.alertmanager"]` | The apps (IDs) whose own posts start the assistant (below); at most 50, none twice, `[]` for none. |
| `AppWindowsPerChannelHour` | 30 | The most windows apps' posts start in a channel per hour (0: none). |

A pattern's `match` expressions are regular expressions over the message with its entities as slots: `{host}`,
`{host|service}`; named groups (`(?P<metric>cpu|memory)`) are words templates can use as `{$metric}`. Built-in
entity types: `host`, `service`, `dashboard`, `repo` (from the dictionary), `person` (@mentions), `ticket`
(`ABC-123`), `number` (`#12`, `12`), `date` (today, weekdays, `2026-10-24`).

## Which messages start it

A person's new message: not a system message, a webhook's, a command's response, a bot's, an app's or another
server's, nor one posted from a card. Edits never start it.

A trusted app's new post, too (`TriggeringApps`), such as the Alertmanager app's alerts:

- Its author is a bot whose owner, on the bot's record (read by the server, never a prop), is one of those apps, and
  the app runs. It came through the apps' API (`from_plugin` and `from_bot`, which only the server sets), as a plain or
  attachment post, without `override_username`, `override_icon_url`, `override_icon_emoji` or
  `webhook_display_name`. Incoming webhooks, commands' responses, OAuth apps, other bots (a person's own, or an app's
  not in the list) and system messages never count; a person's message with `from_plugin` or an override set is never
  an app's: it's taken as before (with `from_plugin`, it doesn't start it).
- The engines read its text, then each attachment's pretext, title, text and fields (`title: value`), a line each, at
  most 2000 characters. An app updating its post (an alert that resolves) doesn't start it.
- Its insights are each viewer's own, run as them, as for any message. An action from it has no author: it's offered
  to those who spoke after it in the window, then by who opened the assistant first. The card's `why.app` is the app.
- The budgets and the debounce are the usual ones; a window with apps' posts also counts on its channel's
  `AppWindowsPerChannelHour`: past it, the apps' posts are left out of the window (people's still count).
