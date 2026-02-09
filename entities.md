I have clarified what I want for the data layer of wydo.

There are a few key entities that wydo handles:

1. Projects
2. Tasks
3. Boards & Cards
4. Notes

## Notes

A note is any markdown file that is NOT a board or card. Notes can have the same front matter fields as cards. See the "Card Frontmatter" section below

## Boards & Cards

Boards and Cards are markdown files. A Board is composed of Cards. Boards live inside directories named `boards/`. Each immediate subdirectory within a `boards/` directory that contains a `board.md` file is a board. The `board.md` file is the board's index — it manages the "columns" of the board and the positions of Cards. For example:

```md
# dev-work

## To Do

## Next

## In Progress

## Test

## Review

[CC - Outstanding Balance](./cards/cc_outstanding_balance.md)

[External API Spec Strategy](./cards/external_api_spec_strategy.md)

## Deploy

## Done

[OD Read-only audit](./cards/od_read_only_audit.md)

```

The "columns" are stored as h2 headers in the md file and are used for tracking cards. Links to cards are relative, typically pointing to files in a `cards/` subdirectory. Cards are markdown files linked from `board.md` that are NOT the index file itself.

There should not be any card in the board's directory that is not tracked by the `board.md` file. Meaning ALL cards should be tracked.

### Card Frontmatter

Cards have a few key frontmatter fields:
- `due` is the card's due date. This is stored in yyyy-mm-dd format.
- `scheduled` is the card's scheduled date. This is stored in yyyy-mm-dd format.
- `date` is a general date marker. This is stored in yyyy-mm-dd format.
- `projects` is a list of projects linked to the card. The identifier for a project is the directory name of the project.
- `tags` is a list of tags on the card. This is just a list of strings.
- `url` is a web url. can be launched from the board view

## Projects

Projects are tracked by a special directory name: `projects/` (similar to boards/cards). A project is a subdirectory within a `projects/` folder. For example `projects/home-remodel/` would represent the `home-remodel` project. The `home-remodel` can contain notes, projects, boards, and tasks. This means that projects can have their own boards, tasks, and projects can have sub-projects as well.

## Tasks

Tasks are tracked in special directories named `tasks/` (similar to projects). They typically contain todo.txt and done.txt files. However, they can contain other .txt files that behave similarly. No other files should exist in a `tasks/` directory. todo.txt files adhear to the [todo.txt format](https://github.com/todotxt/todo.txt)

Tasks can be linked to projects with `+` for example `buy lumber +home-remodel`, this links the task to a project.

Tasks have context tags as well with `@` like `@work`

and there are also key-value tags. These are used for tracking due/scheduled dates. For example `buy lumber +home-remodel due:2026-02-15 scheduled:2026-02-12`
