# Design: Scroll to Moved Card

**Date:** 2026-02-27

## Problem

When moving a card left or right between columns (`h`/`l` in move mode), the destination column does not scroll to show the moved card. The card is appended to the bottom of the destination column, but if that column is scrolled to the top, the card is out of view.

## Solution

In `updateMove` (`internal/tui/kanban/board.go`), call `adjustScrollPosition()` after `adjustHorizontalScrollPosition()` in both the `"h"` and `"l"` cases.

This reuses the existing scroll logic that already handles the "ensure selected card is visible" invariant and is consistent with how every other selection change works (e.g., `j`/`k` reorder already calls `adjustScrollPosition()`).

## Changes

- `internal/tui/kanban/board.go`: Add `m.adjustScrollPosition()` in the `"h"` case and `"l"` case of `updateMove`, after `m.adjustHorizontalScrollPosition()`.

## Scope

Two lines added. No new logic, no new state.
