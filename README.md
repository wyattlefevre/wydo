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
  "dirs": ["~/wydo"],
  "recursive_dirs": [],
  "todo_file": "todo.txt",
  "done_file": "done.txt",
  "default_view": "day"
}
```

| Field | Description | Default |
|-------|-------------|---------|
| `dirs` | Directories to scan for `board.md` and `todo.txt` files | `~/wydo` |
| `recursive_dirs` | Root directories to recursively search | `[]` |
| `todo_file` | Filename for todo tasks | `todo.txt` |
| `done_file` | Filename for completed tasks | `done.txt` |
| `default_view` | Initial TUI view (`day`, `week`, `month`, `tasks`, `boards`) | `day` |

Config priority: CLI flags > environment variables > config file > defaults.

Environment variables: `WYDO_DIRS`, `WYDO_RECURSIVE_DIRS` (colon-separated).

## TUI

```
wydo                        # launch with default view
wydo --view week            # launch in week view
wydo -d ~/projects          # scan specific directories
wydo -r ~/projects          # recursively scan directories
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
