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
