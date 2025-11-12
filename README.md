# Role Bot

A simple Discord bot written in Go that assigns and removes roles when users react to messages with specific emojis. Each channel can have its own configuration, and the bot automatically updates role posts when you modify its configuration file.

---

## Overview

This bot allows users to self-assign roles through emoji reactions on a single message. It supports multiple channels, Unicode and custom emojis, and live configuration reload without restarting the bot.

---

## Features

- Config-based setup (no recompilation required)
- Multiple channel support
- Automatic message updates when the configuration changes
- Custom and Unicode emoji support
- Logging to `logs.txt` in the configuration directory
- Secure token management with `--token-file`

---

## Requirements

- Go 1.21 or newer
- Discord bot token

### Recommended Bot Permissions

- Manage Roles
- Read Messages/View Channels
- Add Reactions
- Read Message History
- Send Messages

**Note:** The bot's highest role must be above any roles it manages in the server hierarchy.

---

## Configuration

### Default Config Path
```
$HOME/.config/role-bot/config.json
```

If no configuration file is found, the bot creates a default one similar to this:

```json
{
  "bot_token": "PUT_YOUR_TOKEN_HERE",
  "channels": [
    {
      "channel_id": "YOUR_CHANNEL_ID_HERE",
      "message_id": "",
      "roles": [
        {
          "emoji": "🔥",
          "role_id": "ROLE_ID_HERE",
          "label": "Example Role"
        }
      ]
    }
  ]
}
```

### Field Descriptions

| Field | Description |
|-------|--------------|
| `bot_token` | Discord bot token (ignored if using `--token-file`) |
| `channels` | List of channels for role assignment posts |
| `channel_id` | Discord channel ID |
| `message_id` | The message ID (autofilled when bot creates a message) |
| `roles` | List of emoji and role mappings |
| `emoji` | Unicode or custom emoji string (e.g. `<:name:id>`) |
| `role_id` | Discord role ID |
| `label` | Human-readable name for the role |

---

## Running the Bot

### Basic Run
```bash
go run main.go
```

### Custom Config Path
```bash
go run main.go --config /path/to/config.json
```

### Token File
```bash
go run main.go --token-file /path/to/token.txt
```

Ensure your token file has secure permissions:
```bash
chmod 600 /path/to/token.txt
```

---

## Live Reload

The bot watches the configuration directory for changes. Editing and saving the JSON file (e.g., adding or removing a role) triggers an automatic reload. Existing reactions and user roles remain intact.

---

## Logs

Logs are saved to:
```
$HOME/.config/role-bot/logs.txt
```

The bot logs all major actions, including:
- Message creation and updates
- Role assignments and removals
- Config reloads
- Errors and warnings

---

## Building

To build a binary:
```bash
go build -o role-bot
./role-bot
```

To cross-compile:
```bash
GOOS=linux GOARCH=amd64 go build -o role-bot
```

---

## Libraries Used

- [discordgo](https://github.com/bwmarrin/discordgo) — Discord API client for Go
- [fsnotify](https://github.com/fsnotify/fsnotify) — File system watcher for live reload

---

## License

This project is released under the MIT License.
