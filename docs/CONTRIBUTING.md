# Contributing

Thanks for looking. zen-notes is small on purpose, so the most useful thing to
read first is the scope section at the bottom.

## Setup

```
git clone https://github.com/praxis-labs-io/zen-notes.git
cd zen-notes
git config core.hooksPath .githooks
make install
```

`.githooks/pre-push` rejects pushes to `main`. It lives in the repo because
untracked `.git/hooks/` files don't survive a clone. Don't reach for
`--no-verify`.

`make install` builds this tree into `~/.local/bin/zen-notes`. Run it after a
change or you keep testing the old binary. To try it:

```
ZEN_NOTES_DIR=/tmp/zn zen-notes
```

Point `ZEN_NOTES_DIR` at a scratch directory while you work. The app writes on
a timer with no confirmation, and it is your real notes otherwise.

## The checks

`make all` is the gate. It should be clean before you open a pull request.

| Command | Does |
| --- | --- |
| `make all` | lint, test, build |
| `make lint` | gofmt, `go mod tidy`, `go vet`, golangci-lint |
| `make test` | `go test -race` with coverage |
| `make fmt-fix` | `gofmt -w .` |
| `go test ./internal/editor -run TestName` | a single test |

`make help` lists every target.

Run checks directly, never through a pipe that swallows the exit code.
`gofmt -l .` exits 0 even when it lists files, and `golangci-lint run | tail`
reports success on failure.

CI runs the same checks plus a cross-compile to five targets: darwin and linux
on amd64 and arm64, and windows on amd64. golangci-lint is pinned in
`.github/workflows/ci.yml` to the local brew version. Bump both together, or CI
and local runs stop agreeing.

## Boundaries

Breaking one of these is a review-stopper.

- **`internal/note` is storage.** Days, paths, atomic save and the directory
  watch. It knows nothing about the editor.
- **`internal/editor` is the vim editor and the renderer.** It takes keys and
  returns a rendered frame, and knows nothing about files or Bubble Tea
  messages.
- **`internal/app` is the only place the two meet**, and the only package that
  talks to the terminal.
- **`internal/version`** holds the version release builds stamp in.

Agent-facing invariants live in [`CLAUDE.md`](../CLAUDE.md).

## Design constraints worth knowing before you change something

**The buffer is hand-rolled, and has to be.** `bubbles/textarea` has no
per-character styling hook and no modal editing. Syntax highlighting and vim
motions both need to own the buffer and the render path.

**Build order in the renderer is load-bearing.** A logical line becomes styled
runes, then wraps into visual rows, then the cursor is positioned. Doing it in
that order is what keeps wrapping, highlighting and the cursor agreeing. Any
approach that counts ANSI bytes as columns will look right and be wrong.

**Motions resolve to a target and a kind; operators consume that.** `dw`,
`d2w`, `3j` and `dd` are one code path, not four special cases. If you are
adding a motion, add it to the resolver and it works under every operator for
free.

**The watcher watches the directory, not the file.** An atomic save renames a
new inode over the target, and a watch on the file follows the old one.

**No hardcoded hex.** Everything is an ANSI slot so it inherits the terminal
theme. The four exceptions are computed from the terminal's own background
color, and there are fallbacks for terminals that will not report it.

## Test conventions

Editor tests drive the public key entry point and assert on the buffer, the
cursor or the rendered output. A test that reads a mode flag or an internal
field can stay green while the thing it claims to cover is broken.

Key sequences use vim notation so a test reads like typing:

```go
e := run(t, "one two three", "ciwzap<esc>")
if e.Text() != "zap two three" {
    t.Fatalf("Text = %q, want zap two three", e.Text())
}
```

Bare runes are literal. `<esc>`, `<cr>`, `<bs>`, `<c-d>` and friends are named.
`<lt>` is a literal `<`, so `<` stays typable.

When you are adding vim behavior, check it against real vim before you trust
your expectation.

Confirm a new test actually catches its regression. Break the code, watch it
fail, put it back.

## Verifying by hand

Unit tests cannot see the terminal, and a few classes of bug only exist there:
cursor shape, real key encodings, glyph width, whether the theme query gets an
answer. Drive the app under tmux for those.

```
tmux new-session -d -s zn -x 80 -y 24 'ZEN_NOTES_DIR=/tmp/zn zen-notes'
tmux send-keys -t zn 'ihello' Escape
tmux capture-pane -pt zn
```

Three traps:

- `tmux send-keys` treats `;` as a command separator. Send it with
  `tmux send-keys -t zn -l '\;'` or it looks like a broken binding.
- Escape followed immediately by another key is parsed as a meta sequence, so
  `Escape` then `k` arrives as Alt+k and never leaves insert mode. Sleep
  between them.
- `capture-pane` normalizes some output. For escape sequences, use
  `tmux pipe-pane` and read the raw bytes.

Manual checks worth running for anything touching save, watch or layout:

- Two terminals on the same day. Type in one, the other updates within a
  second. Then edit both at once and confirm the conflict behaves as
  [the guide](guide.md) describes, with no crash and no garbled file.
- A long line at a narrow width, to confirm wrapping and the cursor still
  agree.
- Light theme and dark theme, and a theme switch while the app is running.

## Docs that describe the code

Every user-facing surface has a document that describes it, and a change to the
surface makes that document wrong until somebody moves it. This is the map, read
at merge time and again before a release:

| Changed | Read |
| --- | --- |
| `internal/editor/**` | [`keys.md`](keys.md) |
| `internal/app/**` | [`keys.md`](keys.md), [`guide.md`](guide.md) |
| `internal/note/**` | [`guide.md`](guide.md) |
| `main.go`, `install.sh`, `.github/workflows/**` | [`install.md`](install.md), [`README.md`](../README.md) |
| the test conventions, the boundaries | this file |

`git diff --name-only <ref>..HEAD` gives the left column, so the set of documents
to check is derived rather than remembered.

A change nothing on this map covers is one of two things: a doc gap to fill, or
work no user sees. Say which rather than leaving it unanswered.

## Version numbers

Semver, and pre-1.0 while the shape can still move:

- **Minor** carries anything a user would notice. A new binding, a changed
  default, a note that lands somewhere else.
- **Patch** carries fixes and everything internal.
- **Major** waits for 1.0.

A published tag is permanent. It cannot be renumbered, and a release cut under
the wrong number stays wrong, so the version is confirmed before the tag is
pushed rather than inferred from the range.

Releases are cut with the `release` skill, which curates
`docs/release-notes/vX.Y.Z.md` and hands the tag command over. The notes file has
to be on `main` before the tag is cut: the workflow reads it out of the tagged
commit.

## Pull requests

- One change per pull request. Do not bundle adjacent cleanup.
- Say what you verified and what it showed, not that you tested it.
- Commit subjects are imperative and plain: `Add screen motions, scroll
  positioning, and the vim staples`.
- No comments inside a function body. Outside one, only three kinds: a
  one-line file purpose when the name doesn't say it, a doc comment on an
  exported name, and a one-line why on a declaration when the code can't show
  it. If code needs more, fix the naming or the structure.
- No `TODO` or `FIXME`. Out-of-scope follow-ups belong in an issue.

## Scope

zen-notes is a notepad, not an editor. The bar for a new feature is that a
person opening a terminal to write down one thing needs it.

Wanted:

- Vim behavior a daily vim user would expect and miss. `.` repeat, marks,
  named registers, macros and the jumplist are all fair game.
- Bugs in wrapping, highlighting, the cursor, or the sync path.
- Terminal compatibility fixes, especially theme and key reporting.

Not wanted, and these have been considered:

- A configuration file. The colors come from your terminal, and the keys are
  vim's.
- Multiple notes per day, folders, tags, or a title. One day is one file.
- A note list, a picker, or a sidebar. `[` and `]` walk the days.
- Anything that reaches the network. Sync is your sync service's job, which is
  why the storage directory is a variable.

Open an issue before a large change. A pull request that gets rejected on
scope is a waste of your evening.
