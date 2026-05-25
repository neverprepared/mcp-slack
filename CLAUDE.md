# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

```bash
# Install (editable) into a uv-managed venv
uv venv && uv pip install -e .

# Smoke test: instantiate the server (a fake bot token is enough — no Slack call is made at startup)
SLACK_BOT_TOKEN=xoxb-test uv run python -c "from mcp_slack.server import create_server; create_server()"

# Run the server (stdio transport — what an MCP host launches)
SLACK_BOT_TOKEN=xoxb-... uv run mcp-slack
# equivalently: uv run python -m mcp_slack

# Tests (pytest is configured in pyproject.toml; no tests exist yet)
uv run pytest
uv run pytest tests/path/to/test_file.py::test_name
```

There is no lint/typecheck config in this repo.

## Architecture

This is a **FastMCP server** that exposes Slack's Web API as MCP tools, with two token sources (OAuth env var, or browser-scraped tokens relayed through Ably).

### Token sourcing — two modes

`SlackClient` auto-selects mode in `client.py`:

| Mode | Trigger | Path |
|---|---|---|
| `oauth` | `SLACK_BOT_TOKEN` env set | `WebClient(token=env)` — never touches cache or keychain |
| `relay` | env unset, `TokenCache` provided | Lazy-build `WebClient` from `cache.get()` (token + `d` cookie header). `AblySubscriber` pushes refreshes via `client.refresh_from_token(payload)` |

The `bot` attribute is a **property** with a lock — never assign to it; use `refresh_from_token`. Tests that want to inject a user-token mock should set `cli._user` (the backing field), not `cli.user`.

### Relay-mode data flow

```
Chrome ext (chrome-extension/slack/) ──AES-256-GCM──► Ably channel
                                                          │
              ┌───────────────────────────────────────────┘
              ▼
        AblySubscriber (background thread, own asyncio loop)
              │  history-on-start → live subscribe
              ▼
        on_token(payload):
          1. TokenCache.save() — atomic write to $XDG_CONFIG_HOME/mcp-slack/token.enc (0600)
          2. SlackClient.refresh_from_token() — swap in-memory WebClient
```

Crucial invariants:
- `crypto.py` (Python) and `chrome-extension/slack/crypto.js` are byte-compatible: PBKDF2-HMAC-SHA256, 100 000 iters, AES-256-GCM, wire format `base64(salt[32] || nonce[12] || ct+tag)`. **If you change one, change both** or the relay breaks silently.
- Passphrase and Ably API key live in the OS keyring (`secrets.py`, service `mcp-slack`). They are *never* written to disk.
- `config.json` only holds non-secrets (currently just `ably_channel`).
- `XDG_CONFIG_HOME` is honored by `cache.config_dir()` — always use that helper, never hardcode `~/.config`.
- The Ably subscriber runs in a dedicated daemon thread with its own event loop. FastMCP's stdio loop is left alone. `atexit` calls `subscriber.stop()`.

### Three layers (tool registration)

1. **`server.py`** — `create_server()` returns `(FastMCP, SlackClient, AblySubscriber | None)`. Subscriber is `None` in OAuth mode or relay mode without a channel. `atexit` hook stops the subscriber on shutdown. To add a new tool module, write `register_X_tools` and add one line to `create_server`.

2. **`client.py`** — see "Token sourcing" above. Constructor only validates env / cache wiring — it does NOT call Slack. Auth failures surface lazily on first tool invocation. Tools that require the user token must call `client.require_user()` (raises `RuntimeError` with a clear message if missing).

3. **`tools/*.py`** — one module per Slack API domain (messages, channels, users, files, search, reactions, pins, misc). Every tool follows the same shape:

   ```python
   @server.tool()
   @slack_tool                    # from _util — order matters: @server.tool() outermost
   def tool_name(...) -> str:
       """Docstring becomes the MCP tool description shown to the LLM."""
       resp = bot.some_method(...)
       return ok(key=resp["key"])  # JSON string
   ```

   - `@slack_tool` catches `SlackApiError` / `ValueError` / `RuntimeError` and returns `err(...)` JSON. Don't add per-tool try/except — let the decorator handle it.
   - All tools return JSON strings via `ok(**fields)` / `err(message)` from `_util.py`. Never return raw dicts or `SlackResponse` objects (the latter isn't JSON-serializable).
   - Tool docstrings are part of the API surface — they're what the LLM client sees when deciding which tool to call. Keep arg descriptions precise (especially ID formats like `C…`/`U…`/`xoxp-…`).

## Domain conventions

- **Search APIs** (`search.messages`, `search.files`, `search.all`) are user-token only — Slack does not allow bots to call them. The `search.py` module always uses `client.require_user()`.
- **`search_users`** is a **client-side filter** over paginated `users.list` (no native Slack endpoint exists). It walks the entire user directory; if a workspace ever gets large enough for this to matter, add caching rather than removing the tool.
- **File uploads** use `files_upload_v2` (v1 is deprecated by Slack). The v2 response shape returns `files` (list) or sometimes `file` (single) — `upload_file` normalizes both into a `files` list.
- **Block Kit / attachments** are passed as JSON *strings* (`blocks_json`, `attachments_json`), not dicts. MCP tool args are typed by FastMCP from Python type hints; nested dict args don't round-trip cleanly across all MCP clients, so we accept strings and `json.loads` them inside the tool.
- **Pagination**: tools that paginate return `next_cursor` (cursor-based, conversations.*/users.*) or `paging` (page-based, files.*/reactions.list) — mirror whatever Slack returns rather than inventing a unified shape.
