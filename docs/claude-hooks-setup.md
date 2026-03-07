# Claude Code Hooks Setup

wydo can display a badge in the board view showing whether a Claude Code session in the current tmux session is running or waiting for input.

## How It Works

The `scripts/claude-status.sh` script is called by Claude Code lifecycle hooks. It reads the current tmux session name and writes a status file to `~/.config/wydo/claude-status/<session-name>`. wydo polls that directory and renders a badge based on the file contents.

Badge states:
- Purple — Claude is running (processing a prompt)
- Yellow — Claude is waiting for input

## Status File Location

```
~/.config/wydo/claude-status/<tmux-session-name>
```

The file contains a single string: `running` or `waiting`. When the Claude session ends, the file is removed.

## Hook Configuration

Add the following to `~/.claude/settings.json`:

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

Replace `/path/to/wydo` with the absolute path to your wydo checkout.

## Prerequisites

- wydo must be running inside a tmux session. The script exits silently when `$TMUX` is not set.
- The script must be executable: `chmod +x scripts/claude-status.sh`
