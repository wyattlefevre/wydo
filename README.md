# wydo

Unified agenda, kanban, and task manager TUI.

## Install

```
make install
```

## Configuration

Config file: `~/.config/wydo/config.json` (created on first run)

```json
{
  "workspaces": ["~/wydo"],
  "default_view": "day"
}
```

| Field | Description | Default |
|-------|-------------|---------|
| `workspaces` | Workspace directories to recursively scan for entities | `~/wydo` |
| `default_view` | Initial TUI view (`day`, `week`, `month`, `tasks`, `boards`) | `day` |

Config priority: CLI flags > environment variables > config file > defaults.

Environment variable: `WYDO_WORKSPACES` (colon-separated).

Each workspace is recursively scanned for entities by directory convention: `boards/`, `tasks/`, `projects/`. See `entities.md` for details.

## TUI

```
wydo                        # launch with default view
wydo --view week            # launch in week view
wydo -w ~/projects          # scan specific workspace directories
```

### Keybindings

| Key | Action |
|-----|--------|
| `1` | Day agenda |
| `2` | Week agenda |
| `3` | Month agenda |
| `t` | Task manager |
| `b` | Boards |
| `?` | Help overlay |
| `q` | Quit |

## Claude Code Integration

wydo can show a badge on kanban cards indicating whether a linked Claude Code session is running or waiting for input.

### Hook Setup

Add the following hooks to `~/.claude/settings.json` (create the file if it doesn't exist), replacing `/path/to/wydo` with the absolute path to your wydo install:

```json
{
  "hooks": {
    "Stop": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/wydo/scripts/claude-status.sh waiting"
          }
        ]
      }
    ],
    "UserPromptSubmit": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/wydo/scripts/claude-status.sh running"
          }
        ]
      }
    ],
    "SessionEnd": [
      {
        "matcher": "",
        "hooks": [
          {
            "type": "command",
            "command": "/path/to/wydo/scripts/claude-status.sh end"
          }
        ]
      }
    ]
  }
}
```

The hook script requires tmux. It exits silently when `$TMUX` is not set, so it is safe to leave configured even when running Claude Code outside of tmux.

Status files are written to `~/.config/wydo/claude-status/<tmux-session-name>` and cleaned up automatically on session end.

## CLI

```
wydo task add "Buy groceries +home @errands"
wydo task list
wydo task list --all
wydo task list -p project
wydo task list -c context
wydo task list --done
wydo task done <task-id>
wydo task delete <task-id>
```

Aliases: `add`/`a`, `list`/`ls`/`l`, `done`/`do`/`d`, `delete`/`rm`/`del`.
