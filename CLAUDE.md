# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Overview

`mcp-slack` is a **Go** MCP (Model Context Protocol) server that exposes the Slack Web API as MCP
tools over **stdio**. It is built on [`mark3labs/mcp-go`](https://github.com/mark3labs/mcp-go) and
[`slack-go/slack`](https://github.com/slack-go/slack), with a Cobra CLI front end.

Two token sources are supported: a normal OAuth bot token from the environment, or browser-scraped
session tokens relayed from a bundled Chrome extension through an Ably channel (encrypted).

> The `chrome-extension/slack/` directory ships with the repo and is zipped as a release artifact.
> A Python implementation existed historically; it is gone — the `.gitignore` still carries legacy
> Python entries.

## Commands

```bash
# Build
go build ./...
go build -o mcp-slack ./cmd/mcp-slack

# Vet / format (no golangci-lint config in this repo)
go vet ./...
gofmt -l .

# Tests — there are currently NO *_test.go files in this repo.
go test ./...

# Run the MCP server on stdio (default when no subcommand is given)
SLACK_BOT_TOKEN=xoxb-... go run ./cmd/mcp-slack
go run ./cmd/mcp-slack serve

# Interactive setup: store Ably API key, encryption passphrase (OS keychain)
# and ably_channel (config.json) for relay mode
go run ./cmd/mcp-slack setup

# Release (tag push -> .github/workflows/release.yaml -> goreleaser, macOS arm64 only)
git tag v0.1.0 && git push origin v0.1.0
```

### Environment variables

| Var | Purpose |
|---|---|
| `SLACK_BOT_TOKEN` | `xoxb-…`. If set, selects **oauth** mode. |
| `SLACK_USER_TOKEN` | `xoxp-…`. Optional; required in oauth mode for `search_*`, `set_user_profile`, `set_user_presence`. |
| `ABLY_CHANNEL` | Fallback for `ably_channel` when it is absent from `config.json`. |
| `XDG_CONFIG_HOME` | Base for the config dir (default `~/.config`). Always use `cache.ConfigDir()`, never hardcode. |

## Architecture

```
cmd/mcp-slack/main.go     Cobra CLI: root/`serve` -> server.New + server.ServeStdio; `setup` wizard
internal/server           New(version) wires MCPServer + SlackClient + Ably subscriber; Stop()
internal/client           SlackClient — mode selection, RWMutex-guarded bot/user clients
internal/tools            8 modules, 53 registered MCP tools; util.go has wrap()/okJSON()/arg helpers
internal/ably             Background Subscriber goroutine: history-on-start, then live subscribe
internal/cache            TokenCache (encrypted token.enc, 0600, atomic write) + config.json
internal/crypto           PBKDF2-HMAC-SHA256 / AES-256-GCM (mirror of chrome-extension crypto.js)
internal/secrets          OS keyring (zalando/go-keyring), service "mcp-slack"
chrome-extension/slack    MV3 extension that scrapes and publishes encrypted Slack tokens to Ably
```

### Token sourcing — two modes

`client.New` auto-selects in `internal/client/client.go`:

| Mode | Trigger | Behaviour |
|---|---|---|
| `oauth` | `SLACK_BOT_TOKEN` set | `slack.New(botToken)` immediately; never touches cache or keychain. |
| `relay` | env unset, `TokenCache` provided | `Bot()` lazily builds a client from `cache.Get()` (token + `d=` cookie via a custom `RoundTripper`). `Subscriber` pushes refreshes into `RefreshFromToken`. |

- `bot`/`user` are unexported and guarded by an `sync.RWMutex`. Never assign directly — go through
  `Bot()`, `User()`, `RefreshFromToken()`.
- `User()` in **relay** mode falls back to `Bot()` (a scraped `xoxc-` token *is* a user session).
  In **oauth** mode it errors unless `SLACK_USER_TOKEN` is set.
- The constructor never calls Slack. Auth failures surface lazily on the first tool invocation.

### Relay-mode data flow

```
Chrome ext (chrome-extension/slack/) --AES-256-GCM--> Ably channel
                                                          |
        ably.Subscriber (goroutine; history-on-start, then live subscribe)
                                                          |
        onToken(payload):
          1. TokenCache.Save()      -> atomic 0600 write of $XDG_CONFIG_HOME/mcp-slack/token.enc
          2. SlackClient.RefreshFromToken()  -> swap in-memory *slack.Client
```

Invariants:

- `internal/crypto/crypto.go` and `chrome-extension/slack/crypto.js` are byte-compatible:
  PBKDF2-HMAC-SHA256, **100 000** iterations, AES-256-GCM, wire format
  `base64(salt[32] || nonce[12] || ciphertext+tag)`. **Change one, change both** or the relay
  breaks silently.
- The passphrase and Ably API key live in the OS keychain (`internal/secrets`, service
  `mcp-slack`). They are never written to disk.
- `config.json` holds non-secrets only (currently just `ably_channel`).
- `server.New` waits up to 5s (`Subscriber.WaitReady`) so the first tool call has a token; it logs
  a warning and continues on timeout. `Server.Stop()` cancels the subscriber goroutine.

### Tool registration

`internal/server.New` calls eight `Register*Tools(mcp, client)` functions. To add a tool module,
write `RegisterXTools` in `internal/tools` and add one line to `New`.

Every tool follows the same shape:

```go
s.AddTool(mcp.NewTool("tool_name",
    mcp.WithDescription("Shown to the LLM — keep ID formats (C…/U…/xoxp-…) precise."),
    mcp.WithString("channel", mcp.Required(), mcp.Description("…")),
), wrap("tool_name", func(req mcp.CallToolRequest) (string, error) {
    bot, err := c.Bot()   // or c.User() for user-token-only endpoints
    if err != nil { return "", err }
    ...
    return okJSON(map[string]any{"ts": ts}), nil
}))
```

- `wrap()` converts every returned error into an `errJSON` **tool result**, not an MCP protocol
  error. Don't add per-handler error plumbing — return the error and let `wrap` render it.
- All tools return JSON strings via `okJSON(...)` (adds `"status":"success"`) / `errJSON(msg)`.
- Arguments arrive as `map[string]any`; JSON numbers are `float64`. Use the `util.go` helpers —
  `strArg`, `strDefault`, `boolDefault`, `intDefault`, `optStr` — rather than hand-casting.
- Tool descriptions are the API surface the LLM sees. Keep them accurate.

## Tools (53)

| Module | Tools |
|---|---|
| `messages.go` (9) | `post_message`, `update_message`, `delete_message`, `post_ephemeral`, `schedule_message`, `list_scheduled_messages`, `delete_scheduled_message`, `get_permalink`, `get_thread_replies` |
| `channels.go` (16) | `list_channels`, `get_channel_info`, `get_channel_history`, `create_channel`, `archive_channel`, `unarchive_channel`, `rename_channel`, `set_channel_topic`, `set_channel_purpose`, `join_channel`, `leave_channel`, `invite_to_channel`, `kick_from_channel`, `list_channel_members`, `open_dm`, `close_dm` |
| `users.go` (8) | `list_users`, `get_user_info`, `lookup_user_by_email`, `search_users`, `get_user_presence`, `get_user_profile`, `set_user_profile`*, `set_user_presence`* |
| `files.go` (4) | `upload_file`, `list_files`, `get_file_info`, `delete_file` |
| `search.go` (3) | `search_messages`*, `search_files`*, `search_all`* |
| `reactions.go` (4) | `add_reaction`, `remove_reaction`, `get_reactions`, `list_user_reactions` |
| `pins.go` (3) | `pin_message`, `unpin_message`, `list_pins` |
| `misc.go` (7) | `auth_test`, `get_team_info`, `list_emoji`, `list_bookmarks`, `add_bookmark`, `edit_bookmark`, `remove_bookmark` |

`*` = calls `client.User()`; needs `SLACK_USER_TOKEN` in oauth mode (works via the session token in
relay mode). No MCP *resources* or *prompts* are registered — tools only
(`server.WithToolCapabilities(true)`).

## Domain conventions

- **Search APIs** (`search.messages`, `search.files`, `search.all`) are user-token only; Slack does
  not allow bots to call them.
- **`search_users`** is a client-side substring filter over paginated `users.list` — no native
  Slack endpoint exists. If workspace size ever makes this hurt, add caching rather than dropping
  the tool.
- **File uploads** use `files_upload_v2` (v1 is deprecated by Slack).
- **Block Kit / attachments** are passed as JSON *strings* (`blocks_json`, `attachments_json`),
  not nested objects — nested object args don't round-trip cleanly across all MCP clients. They are
  `json.Unmarshal`ed inside the handler.
- **Pagination**: mirror whatever Slack returns — cursor-based `next_cursor` for
  `conversations.*`/`users.*`, page-based `paging` for `files.*`/`reactions.list`. Don't invent a
  unified shape.
- **Logging** goes to stderr via `log` — stdout is the MCP stdio transport and must stay clean.

## Release

`.goreleaser.yaml` builds **darwin/arm64 only** with `CGO_ENABLED=1` (keychain access), stamps
`main.version` via ldflags, publishes a Homebrew formula to `neverprepared/homebrew-tap`
(`Formula/`, needs the `HOMEBREW_TAP_TOKEN` secret), and attaches `chrome-extension.zip` as an
extra release asset. The `release` workflow triggers on `v*` tags and runs on `macos-14`. There is
no CI workflow for build/test on push or PR.
