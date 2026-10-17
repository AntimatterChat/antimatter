# AGENTS.md

Never run `go mod tidy` directly. Always run `make modules-tidy` instead — it excludes private enterprise imports that would otherwise break the tidy.

After editing `i18n/en.json`, always run `make i18n-extract` — it regenerates the file with strings in the required order.

Every entry in `i18n/en.json` carries a `description` explaining where the string appears and how it is used. It is context for translators, human or AI, and the one field a person is expected to write by hand: `make i18n-extract` preserves it, and a newly extracted id arrives with an empty one to fill in. The server never reads it — go-i18n parses only `id` and `translation` and discards the rest — so it is not worth stripping before release.

Prefer request-scoped loggers when logging from request paths. If a method needs to log and does not have access to the request logger, it is reasonable to add `request.CTX` to the method signature when the caller can provide it.
In the store layer, do not use `context.Context` in store method signatures. Use `request.CTX` and only call `rctx.Context()` inside internals that require a standard `context.Context`.

Antimatter's implementations of the `einterfaces` interfaces (AD/LDAP, SAML, the OAuth providers, compliance, data retention, message export, access control, search, auto-translation...) live next to the code they serve (`channels/app/<service>`, `channels/jobs/<job>`, `platform/services/...`) and are built directly where the server starts: `NewServer`, `NewChannels` and `initJobs` in `channels/app`, `initServices` in the platform. Nothing registers itself from `init()`, and those packages never import `channels/app`: the app implements the small backend interface each one declares. When adding one, build it there and extend `TestAntimatterServicesWired` (`channels/app/antimatter_services_test.go`).
