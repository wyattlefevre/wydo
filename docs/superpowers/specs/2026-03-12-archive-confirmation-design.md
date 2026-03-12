# Archive Confirmation Modal

**Date:** 2026-03-12

## Summary

Two small improvements to the Projects view:

1. Show a confirmation modal before archiving a project (pressing `a` when a project is not yet archived).
2. Ensure the `ctrl+a` "toggle show archived" keybind is visible in the `?` help menu.

## Scope

All changes are in `internal/tui/projects/projects.go`. No other files need modification.

The `ctrl+a` keybind is already listed in the `?` help menu (`app.go:1024`) — no change needed.

## Design

### Mode

Add `modeArchiveConfirm` to the `projectMode` enum in `projects.go`.

### State

Add `archiveEntry *projectEntry` to `ProjectsModel`. This holds the project pending confirmation.

### Key Handler (`"a"` case)

- If `newArchived == true` (archiving): set `m.archiveEntry = &entry`, `m.mode = modeArchiveConfirm`. Do not execute the archive yet.
- If `newArchived == false` (unarchiving): execute immediately, same as current behavior.

### Update Handler: `updateArchiveConfirm`

Handles key input when `mode == modeArchiveConfirm`:

- `y` / `Y`: execute `workspace.SetVirtualProjectArchived` or `workspace.SetProjectArchived` depending on project type, rebuild entries, return to `modeList`.
- `n` / `N` / `esc`: clear `archiveEntry`, return to `modeList`.

### View: `viewArchiveConfirm`

Centered full-screen view, same structure as `viewDeleteVirtual`:

```
Archive Project

Archive "my-project"?

[y] Archive   [n/esc] Cancel
```

Error display below if the archive operation fails.

### Hint Text

Add case for `modeArchiveConfirm` in `HintText()`: `"y:archive  n/esc:cancel"`.

### View Dispatch

Add `case modeArchiveConfirm: return m.viewArchiveConfirm()` in the `View()` switch.

## Out of Scope

- No changes to unarchive behavior.
- No changes to the `ctrl+a` toggle or its help entry.
- No changes to any other view or package.
