# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

A one-note-per-day markdown TUI, at `praxis-labs-io/zen-notes` (`origin`). Open
source, MIT. `docs/` holds everything a user reads: the guide, the keymap and
install. `docs/CONTRIBUTING.md` holds the scope rules and what has already been
rejected, and `README.md` is the front page linking in. Read those rather than
restating them here.

**`main` is the product branch.** Feature work flows ticket → branch → PR on
`origin` (see Project Management).

Two things skip the PR and commit straight to `main`:

- Genuinely trivial tweaks. A typo, a one-liner.
- **Doc-only changes with no code.** Markdown, comments, `CLAUDE.md`, rules files. A PR for prose is ceremony.

A tracked pre-push hook rejects pushes to `main`, so an agent commits these and
Drew pushes them. Don't reach for `--no-verify`.

The installed binary is built from here to `~/.local/bin/zen-notes`. **Rebuild
after changes or Drew keeps running the old code:**

```sh
make install
```

Releases are a pushed tag. `.github/workflows/release.yml` builds the five
targets, writes the checksums, and cuts the release from
`docs/release-notes/<tag>.md`, which has to be on `main` before the tag is cut.
`go install github.com/praxis-labs-io/zen-notes@latest` only resolves to a
tagged version, so a release the users can install still means a new tag.
Releases are cut with the `release` skill in `.claude/skills/`, which belongs to
this repo and is not a copy of a global one.

Anything published under Drew's name (PR bodies, issues, release notes, README)
must be shown to him word-for-word before pushing. His voice: terse,
considerate, stoic, no strong adverbs, no em-dashes.

## Conventions

@.claude/rules/code-quality.md

That file holds only the Go and Bubble Tea specifics. The principles and voice
rules are global and load automatically; don't copy them in here, that only
creates drift.

## Commands

The checks, the lint version pin and the hook setup are in
`docs/CONTRIBUTING.md`. `make all` is the gate. The SessionStart hook wires up
`.githooks` on every session, so a fresh clone is covered.

Never run the app against the real notes directory. Use
`ZEN_NOTES_DIR=/tmp/zn zen-notes`.

## Charm module paths

The Charm v2 line lives under `charm.land/*`, not `github.com/charmbracelet/*`.
`github.com/charmbracelet/bubbletea/v2` does not resolve. `charmbracelet/x/ansi`
keeps its github path.

## Architecture

The package boundaries, the renderer's build order, motion resolution, the
directory watch and the test conventions are in `docs/CONTRIBUTING.md`. This
section holds what it doesn't cover and a contributor can break without
noticing.

- **`Buffer` holds `[][]rune` and `Pos.Col` counts runes, not bytes or cells.**
  Display width only enters at render time, through `runewidth`.
- **`decideReload` is the whole conflict policy.** Our own save comes back
  through the watcher, so a disk copy matching what we last wrote is ignored.
  Dirty means keep local and say so. Otherwise reload.
- **The four derived colours are the cursor line, the selection, the yank flash
  and the search match.** Each shifts the terminal's background lightness and
  keeps its hue. Everything else is an ANSI slot 0 to 15.
- **The background is queried at `Init` and again on every `tea.FocusMsg`**, so
  a mid-session theme switch is picked up. Some multiplexers never answer, so
  there are 256-colour fallbacks and the default assumes dark.
- **`translateKey` in `internal/app/app.go` is where terminal keys become
  `editor.Key`.** Shift is not a modifier: a capital arrives with `ModShift`
  set and the capital already in `Text`, and rejecting it drops
  `A I O G P D C V` and `ZZ`. Ctrl is matched on `msg.Code`, and only
  `c-d c-u c-r c-v` exist. Super+c is the one other modifier, mapped to `copy`.
- **Unit tests can't see the terminal.** Cursor shape, real key encodings,
  glyph width and the theme query need the app under tmux, per
  `docs/CONTRIBUTING.md`.

## Project Management

Work is tracked in Linear: Praxis Labs workspace, reached through the
`linear-zen-notes` MCP server declared in `.mcp.json`. This repo's tickets are
the **Zen Notes** team (key `ZNN`, tickets `ZNN-###`). Address projects and
statuses **by name, never a UUID**; ids don't survive workspace moves.

Bucket names are shared across teams in this workspace, so `save_issue`
resolving a bare project name can land on another team's copy and fail the
call. Pass the Zen Notes project id in that one argument when it does.

### Projects

Four long-running buckets. Every ticket belongs to exactly one:

- **Polish & Bugs**: bugs and rough edges in surfaces that already ship. The
  dogfood inbox.
- **Feature Backlog**: net-new capabilities. Ideas live here until promoted.
- **Performance and Code-Quality**: improves the code, no user-visible change.
- **Release & Distribution**: how the binary gets from `main` to a user and
  stays current.

### Tickets

- Every ticket gets the team, exactly one project, a priority, and a status. No
  orphans.
- Create tickets as we go; never dump a full backlog up front.
- PR-sized scoping: 1 ticket = 1 branch = 1 PR as the rule of thumb.
- Keep descriptions lean: clear title, short goal and scope. No boilerplate
  acceptance criteria.
- Use Linear's generated branch name (`gitBranchName` from the MCP), never an
  invented one.
- Reference the ticket id in commits and the PR title/body so Linear auto-links.
- Status ladder: agent drives Backlog → Todo → In Progress. In Review and Done
  are the GitHub integration's; never write those by hand.

### Shipping

Feature-complete work ships via the global `ship-feature` skill: `make all`
green, push, draft PR, Copilot + `/code-review`, triage with no tech debt,
push then mark ready as separate actions. Manual invocation only.

**There is no copy of it in this repo.** Drew's global skills are a symlink into
drucial-dots and load in every repo, so a copy here only shadows the real one
and drifts behind it. Edit the skill at its source. A session that cannot see
the global skill should say so rather than follow a copy nobody maintains.

### Specs and plans

Scratch, never committed. `docs/` describes only what is true today. Durable
context lives in Linear project descriptions and tickets.
