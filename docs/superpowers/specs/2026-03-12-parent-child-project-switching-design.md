# Parent/Child Project Switching in Project Detail View

**Date:** 2026-03-12
**Status:** Approved

## Overview

Add keyboard navigation from the project detail view (`DetailModel`) to the project's parent or a selected child, without returning to the project list first.

## User-Facing Behavior

### Navigate to parent (`[`)

- When viewing a project detail, pressing `[` navigates directly to the parent project.
- If the current project has no parent (it is a root project), the key is a no-op.
- If the parent is found in the registry, emits `OpenProjectMsg` which the app handles identically to opening a project from the list.
- If the parent name is recorded but not found in the registry (e.g. stale data), the key is a no-op.

### Navigate to child (`]`)

- Pressing `]` opens an inline child picker if the current project has children.
- If the project has no children, the key is a no-op.
- The picker is a scrollable list of child project names (direct children only, not descendants).
- Children are sorted alphabetically (case-insensitive), matching the list view sort order.
- Keyboard controls: `j`/`down`, `k`/`up` to navigate; `enter` to select; `esc` to cancel.
- On selection, emits `OpenProjectMsg` for the chosen child.

### Help menu (`?`)

- A `ViewProjectDetail` section is added to `renderHelpOverlay` in `app.go` listing all detail view bindings including the new `[` and `]` keys.
- The bottom hint bar (`HintText`) is updated to include `[:parent  ]:children`.

## Implementation Scope

### `internal/tui/projects/detail.go`

1. Add `detailModeChildPicker` to the `detailMode` enum.
2. Add `detailChildPicker` struct with fields: `entries []*workspace.Project`, `cursor int`, `width int`, `height int`.
3. Add `childPicker *detailChildPicker` field to `DetailModel`.
4. Add `Update` and `View` methods to `detailChildPicker`.
5. In `DetailModel.Update`, dispatch to `updateChildPicker` when mode is `detailModeChildPicker`.
6. In `handleKey`, add:
   - `[` case: look up parent in registry, emit `OpenProjectMsg` if found.
   - `]` case: get children from registry, build and open child picker if non-empty.
7. Update `HintText` to append `  [:parent  ]:children` to the normal mode string.

### `internal/tui/app.go`

1. Add `case ViewProjectDetail:` in `renderHelpOverlay` with bindings for all detail view keys including `[` and `]`.

## Out of Scope

- Navigating to grandparent or deeper ancestors in a single step.
- Showing a breadcrumb trail.
- Navigating to descendants (non-direct children).
- Any changes to the project list view.
