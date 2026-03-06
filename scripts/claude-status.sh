#!/usr/bin/env bash
#
# claude-status.sh - Update wydo with Claude Code session status.
#
# Called by Claude Code lifecycle hooks. Reads the current tmux session
# name and writes a status file to ~/.config/wydo/claude-status/.
#
# Usage:
#   claude-status.sh waiting    (Stop hook)
#   claude-status.sh running    (UserPromptSubmit hook)
#   claude-status.sh end        (SessionEnd hook)

set -euo pipefail

STATUS="${1:-}"
if [[ -z "$STATUS" ]]; then
    echo "Usage: $(basename "$0") waiting|running|end" >&2
    exit 1
fi

# Only run inside tmux
[[ -z "${TMUX:-}" ]] && exit 0

SESSION=$(tmux display-message -p '#S')
STATUS_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/wydo/claude-status"

mkdir -p "$STATUS_DIR"

if [[ "$STATUS" == "end" ]]; then
    rm -f "$STATUS_DIR/$SESSION"
else
    printf '%s' "$STATUS" > "$STATUS_DIR/$SESSION"
fi
