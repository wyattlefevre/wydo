# Priority Badge Styling Design

**Date:** 2026-03-04

## Problem

Priority indicators (`(A)` in tasks, `1` in kanban cards) are styled with foreground color only. They blend into task lines and are easy to miss at a glance.

## Solution

Apply full background-color badges to priority indicators across all views: tasks list, agenda, kanban cards, and the kanban priority input modal.

## Color Palette

Both the letter-based (A–F) and number-based (1–9) systems share the same urgency scale:

| Task | Kanban | Background | Foreground |
|------|--------|------------|------------|
| A    | 1      | magenta (5)  | black (0)  |
| B    | 2      | red (1)      | black (0)  |
| C    | 3      | orange (208) | black (0)  |
| D    | 4      | yellow (3)   | black (0)  |
| E    | 5      | green (2)    | black (0)  |
| F    | 6–9    | dim gray (8) | white (15) |

## Files Changed

1. `internal/tui/shared/task_line.go` — add `taskPriorityStyle(p data.Priority)` helper; use it for `(A)` token
2. `internal/tui/theme/theme.go` — remove unused `Priority` style var
3. `internal/tui/agenda/item_line.go` — split priority badge from title so badge gets background style independently
4. `internal/tui/kanban/priority_input.go` — change `priorityColor()` to return full `lipgloss.Style` with background; update `View()`
5. `internal/tui/kanban/board.go` — update card priority prefix rendering to use bg+fg style
