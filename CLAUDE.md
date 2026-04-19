# CLAUDE.md

This file provides guidance to Claude Code when working in this repository.

## Service / data layer rules

**Always batch service mutations; reload once at the end.**
Never call `Delete()`, `Update()`, or any other mutating service method in a loop — each call writes to disk and triggers `Reload()`, which re-reads every task file. Use the batch variants instead:
- `DeleteByIDs([]string)` — single write + reload for multiple deletes
- `ArchiveByIDs([]string)` — same pattern for archiving

When adding new bulk operations, follow this pattern: accumulate changes in memory, write once, reload once.
