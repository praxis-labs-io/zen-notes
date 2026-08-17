# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this repo is

A one-note-per-day markdown TUI, at `praxis-labs-io/zen-notes` (`origin`). Open
source, MIT. `README.md` is the user-facing description and the full keymap;
`CONTRIBUTING.md` holds the scope rules and what has already been rejected.
Read those rather than restating them here.

`main` is the only branch. There is no Makefile and there are no git hooks.

CI is `.github/workflows/ci.yml`: lint and format, test, and a cross-compile
matrix, on pushes to `main` and non-draft PRs. It runs the same checks listed
below, so a clean local run is a clean CI run.

The installed binary is built from here to `~/.local/bin/zen-notes`. **Rebuild
after changes or Drew keeps running the old code:**

```sh
GOBIN=$HOME/.local/bin go install .
```

Releases are a pushed tag plus `gh release create`. `go install
github.com/praxis-labs-io/zen-notes@latest` only resolves to a tagged version,
so a release the users can install means a new tag.

## Commands

```sh
go build ./...
go test -race ./...
go vet ./...
gofmt -l .                                      # prints nothing on a pass
golangci-lint run ./...
go mod tidy && git diff --exit-code go.mod go.sum
go test ./internal/editor -run TestName         # single test
go test ./internal/editor -run 'CursorLine'     # a group, by regexp
```

All of those clean before anything is committed. `gofmt -l .` exits 0 even when
it lists files, so a `&&` chain will not catch it; read the output.

golangci-lint is pinned to v2.12.2 in the workflow to match the local brew
version. Bump both together or local runs and CI stop agreeing.

Never run the app against the real notes directory. Use a scratch one:

```sh
ZEN_NOTES_DIR=/tmp/zn zen-notes
```

## Charm module paths

The Charm v2 line lives under `charm.land/*`, not `github.com/charmbracelet/*`.
`github.com/charmbracelet/bubbletea/v2` does not resolve. `charmbracelet/x/ansi`
keeps its github path.

## Architecture

Three packages under `internal/`, layered, with `main.go` resolving flags and
the notes directory.

- **`note`** is storage. Days, paths, atomic save, the fsnotify watch. Knows
  nothing about the editor.
- **`editor`** is the whole vim editor and renderer. Takes keys, returns a
  rendered frame. Knows nothing about files or Bubble Tea messages.
- **`app`** is the only place the two meet, and the only package that talks to
  the terminal.

Breaking that is a review-stopper. A `tea.Msg` in `editor` or a `note.Store` in
`editor` means the boundary went.

`CONTRIBUTING.md` lists what lives in each file.

### Invariants worth knowing before changing anything

**The renderer builds in one order: styled runes, then wrap, then cursor
position.** That order is what keeps wrapping, highlighting and the caret
agreeing. Anything that counts ANSI bytes as columns will look right and be
wrong.

**Motions resolve to a target plus a kind; operators consume that result.**
`dw`, `d2w`, `3j` and `dd` are one code path. A new motion added to the
resolver works under every operator for free, and one added anywhere else is a
special case that will not.

**`Buffer` holds `[][]rune` and `Pos.Col` counts runes, not bytes or cells.**
Display width only enters at render time, through `runewidth`.

**The watcher watches the directory, not the file.** An atomic save renames a
new inode over the target, and a watch on the file follows the old one.

**`decideReload` is the whole conflict policy.** Our own save comes back
through the watcher, so a disk copy matching what we last wrote is ignored.
Dirty means keep local and say so. Otherwise reload.

### Colour

No hardcoded hex. Everything is an ANSI slot 0 to 15 so it inherits the
terminal theme. The four exceptions are the cursor line, the selection, the
yank flash and the search match: each is derived from the terminal's own
background by shifting lightness and keeping hue.

**The background is queried at `Init` and again on every `tea.FocusMsg`**, so a
mid-session theme switch is picked up. Some multiplexers never answer, which is
why there are 256-colour fallbacks and why the default assumes dark.

### Key translation

`translateKey` in `internal/app/app.go` is where terminal keys become
`editor.Key`. Two facts it encodes, both learned the hard way:

- **Shift is not a modifier.** A capital arrives with `ModShift` set and the
  capital already in `Text`. Rejecting it drops `A I O G P D C V` and `ZZ`.
- Ctrl is matched on `msg.Code`, and only `c-d c-u c-r c-v` exist.

## Testing

Editor tests drive the public key entry point and assert on the buffer, the
cursor or the rendered output. A test reading a mode flag or an internal field
can stay green while the thing it claims to cover is broken.

Key sequences use vim notation, so a test reads like typing. `<lt>` is a
literal `<`:

```go
e := run(t, "one two three", "ciwzap<esc>")
```

Check new vim behaviour against real vim before trusting the expectation.
Several tests here were wrong on the first pass and the code was right.

### Verifying by hand

Unit tests cannot see the terminal. Cursor shape, real key encodings, glyph
width and whether the theme query gets an answer only exist there. Drive it
under tmux, and mind three traps:

- `tmux send-keys` takes `;` as a command separator. Send it with `-l '\;'`.
- Escape followed immediately by another key is parsed as a meta sequence, so
  `Escape` then `k` becomes Alt+k and never leaves insert mode. Sleep between
  them.
- `capture-pane` trims trailing whitespace, so a full-width background looks
  short. Use `tmux pipe-pane` for raw bytes.

## Project Management

Work is tracked in Linear: Praxis Labs workspace, reached through the
`linear-zen-notes` MCP server declared in `.mcp.json`. This repo's tickets are
the **Zen Notes** team (key `ZNN`, tickets `ZNN-###`). Address projects and
statuses **by name, never a UUID**; ids don't survive workspace moves.

The team is new: no projects and no tickets yet. Bucket names are shared across
teams in this workspace, so `save_issue` resolving a bare project name can land
on another team's copy and fail the call. Pass the Zen Notes project id in that
one argument when it does.

- Every ticket gets the team, exactly one project, a priority, and a status. No
  orphans.
- Create tickets as we go; never dump a full backlog up front.
- Keep descriptions lean: clear title, short goal and scope. No boilerplate
  acceptance criteria.
- Status ladder: agent drives Backlog → Todo → In Progress. In Review and Done
  are the GitHub integration's; never write those by hand.
