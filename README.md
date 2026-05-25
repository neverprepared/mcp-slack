# mcp-slack

MCP server for Slack — full CRUD on messages, channels, users, files, reactions, pins, bookmarks, and workspace search.

## Tools

### Messages
| Tool | Description |
|------|-------------|
| `post_message` | Post to a channel, DM, or thread (text/blocks/attachments) |
| `update_message` | Edit an existing message |
| `delete_message` | Delete a message |
| `post_ephemeral` | Post a message visible to only one user |
| `schedule_message` | Schedule a message for later (<120 days) |
| `list_scheduled_messages` | List pending scheduled messages |
| `delete_scheduled_message` | Cancel a scheduled message |
| `get_permalink` | Get a permanent URL for a message |
| `get_thread_replies` | Fetch replies to a threaded message |

### Channels (conversations)
| Tool | Description |
|------|-------------|
| `list_channels` | List public/private channels, DMs, MPIMs |
| `get_channel_info` | Channel metadata + member count |
| `get_channel_history` | Paginated message history with time filters |
| `create_channel` | Create public or private channel |
| `archive_channel` / `unarchive_channel` | (Un)archive |
| `rename_channel` | Rename |
| `set_channel_topic` / `set_channel_purpose` | Set topic/purpose |
| `join_channel` / `leave_channel` | Membership |
| `invite_to_channel` / `kick_from_channel` | Manage members |
| `list_channel_members` | List member user IDs |
| `open_dm` / `close_dm` | Open/close DMs and MPIMs |

### Users
| Tool | Description |
|------|-------------|
| `list_users` | Paginated workspace user list |
| `get_user_info` | Full user object |
| `lookup_user_by_email` | Find by email |
| `search_users` | Substring search across name/real_name/display_name/email |
| `get_user_presence` | Active/away |
| `get_user_profile` | Profile incl. custom fields |
| `set_user_profile` | Update profile (user token) |
| `set_user_presence` | Set caller's presence (user token) |

### Files
| Tool | Description |
|------|-------------|
| `upload_file` | Upload local file (uses files_upload_v2) |
| `list_files` | List files with user/channel/time/type filters |
| `get_file_info` | File metadata + comments |
| `delete_file` | Delete file |

### Search (requires user token)
| Tool | Description |
|------|-------------|
| `search_messages` | Slack search with modifiers (in:, from:, before:, has:) |
| `search_files` | File search |
| `search_all` | Combined messages + files + posts |

### Reactions
| Tool | Description |
|------|-------------|
| `add_reaction` / `remove_reaction` | (Un)react to a message |
| `get_reactions` | Reactions on a message |
| `list_user_reactions` | Items a user has reacted to |

### Pins & Bookmarks
| Tool | Description |
|------|-------------|
| `pin_message` / `unpin_message` / `list_pins` | Channel pins |
| `list_bookmarks` / `add_bookmark` / `edit_bookmark` / `remove_bookmark` | Channel bookmarks |

### Workspace
| Tool | Description |
|------|-------------|
| `auth_test` | Verify token + return identity |
| `get_team_info` | Workspace info |
| `list_emoji` | Custom emoji catalog |

## Authentication

The server runs in one of two modes (auto-detected):

| Mode | When | Token source |
|---|---|---|
| **OAuth** | `SLACK_BOT_TOKEN` env is set | Long-lived bot/user OAuth tokens |
| **Relay** | No env token | Encrypted on-disk cache, refreshed live by an Ably subscriber from the bundled Chrome extension |

### Relay mode (browser-scraped tokens via Ably)

For workspaces where you can't install an OAuth app — uses `xoxc-` + `d` cookie scraped by the Chrome extension in `chrome-extension/slack/`.

```
Chrome ext ──encrypt──► Ably channel ──► AblySubscriber thread ──► encrypted cache file ──► SlackClient
                                                                          │
                                          macOS Keychain ◄── passphrase + Ably key
```

1. Load `chrome-extension/slack/` as an unpacked extension in Chrome.
2. Click the extension → **Generate Key** (or paste an existing one), enter your **Ably API key** and **channel name**, save.
3. On the MCP host, run `mcp-slack-setup`. Paste the same passphrase, same Ably key, same channel name.
4. Open `app.slack.com` — the extension captures the next token and publishes it.
5. Start the MCP server. It pulls the most recent token from Ably history on boot, then subscribes for live updates.

**Storage:**
- Tokens: `$XDG_CONFIG_HOME/mcp-slack/token.enc` (defaults to `~/.config/mcp-slack/token.enc`), AES-256-GCM encrypted, `0600` perms.
- Non-secret config (channel name): `$XDG_CONFIG_HOME/mcp-slack/config.json`.
- Passphrase + Ably API key: OS keyring (macOS Keychain via the `keyring` library).

The cache survives MCP restarts even if Ably has dropped the message from history — restarts read disk first.

### OAuth mode (recommended when you can install a Slack app)

You need a **Bot Token** (`xoxb-…`). For `search.*` and a few user-only endpoints, also supply a **User Token** (`xoxp-…`).

### Step 1: Create a Slack App
1. Go to [api.slack.com/apps](https://api.slack.com/apps) → **Create New App** → **From scratch**
2. Name it and pick your workspace.

### Step 2: Add Bot Scopes
**OAuth & Permissions** → **Bot Token Scopes**. Add the scopes you need:

| Capability | Scopes |
|---|---|
| Read messages | `channels:history`, `groups:history`, `im:history`, `mpim:history` |
| Send/edit messages | `chat:write`, `chat:write.public` (post to channels bot isn't in) |
| Channels | `channels:read`, `channels:manage`, `groups:read`, `groups:write`, `im:read`, `im:write`, `mpim:read`, `mpim:write` |
| Users | `users:read`, `users:read.email`, `users.profile:read` |
| Files | `files:read`, `files:write` |
| Reactions | `reactions:read`, `reactions:write` |
| Pins | `pins:read`, `pins:write` |
| Bookmarks | `bookmarks:read`, `bookmarks:write` |
| Emoji/team | `emoji:read`, `team:read` |

Click **Install to Workspace** and copy the **Bot User OAuth Token** (`xoxb-…`).

### Step 3 (optional): Add User Scopes for Search
On the same page, add **User Token Scopes**: `search:read`, `users.profile:write` (if you need profile editing). Reinstall the app and copy the **User OAuth Token** (`xoxp-…`).

> Slack's `search.*` family is **only** callable with a user token — bots cannot search. Same for `users.setPresence` and editing other users' profiles.

## Configuration

| Variable | Required | Description |
|---|---|---|
| `SLACK_BOT_TOKEN` | OAuth mode | Bot token (`xoxb-…`). Presence selects OAuth mode. |
| `SLACK_USER_TOKEN` | No | User token (`xoxp-…`) — needed for search and user-only APIs |
| `ABLY_CHANNEL` | No | Override the Ably channel name from `config.json` (relay mode) |
| `XDG_CONFIG_HOME` | No | Override the config dir base (default `~/.config`) |
| `LOG_LEVEL` | No | Logging level (default INFO) |

## Installation

```bash
pip install mcp-slack
```

Or from source:

```bash
git clone https://github.com/neverprepared/mcp-slack
cd mcp-slack
pip install -e .
```

## Claude Code Configuration

Add to your `.claude.json` or `~/.claude.json`:

```json
{
  "mcpServers": {
    "slack": {
      "command": "mcp-slack",
      "env": {
        "SLACK_BOT_TOKEN": "xoxb-...",
        "SLACK_USER_TOKEN": "xoxp-..."
      }
    }
  }
}
```

## License

MIT
