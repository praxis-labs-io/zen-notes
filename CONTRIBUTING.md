# Contributing

Thanks for looking. zen-notes is small on purpose, so the most useful thing to
read first is the scope section at the bottom.

## Build and test

```
go build ./...
go test ./...
go test -race ./...
go vet ./...
gofmt -l .
```

All five should be clean before you open a pull request. `gofmt -l .` printing
nothing is the pass.

To try your build:

```
GOBIN=$HOME/.local/bin go install .
ZEN_NOTES_DIR=/tmp/zn zen-notes
```

Point `ZEN_NOTES_DIR` at a scratch directory while you work. The app writes on
a timer with no confirmation, and it is your real notes otherwise.

## Layout

```
main.go                       flags, resolve the notes dir, run the program
internal/note/store.go        day arithmetic, paths, load, atomic save
internal/note/watch.go        fsnotify directory watch, emits a tea.Msg
internal/editor/buffer.go     lines of runes, edits, clamping
internal/editor/motion.go     word, find and paragraph motions
internal/editor/vim.go        modes, operator-pending, key to action
internal/editor/textobject.go iw aw i" a( ip
internal/editor/visual.go     visual operators, block insert
internal/editor/edits.go      case, replace, gv, %
internal/editor/screen.go     H M L, zt zz zb, viewport
internal/editor/wrap.go       logical lines to visual rows
internal/editor/highlight.go  markdown tokens
internal/editor/search.go     / n N, incremental
internal/editor/render.go     styles, gutter, the rendered frame
internal/app/app.go           tea.Model, autosave tick, reload decisions
internal/app/view.go          status bar, cursor shape, binding list
```

`internal/editor` knows nothing about files or Bubble Tea messages. It takes
keys and returns a rendered frame. `internal/note` knows nothing about the
editor. `internal/app` is the only place the two meet, and it is the only
package that talks to the terminal.

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
theme. The three exceptions are computed from the terminal's own background
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
your expectation. Several of the tests here were wrong on the first pass, not
the code: `d;`, `diw` on punctuation, `di(` from outside the parens, `%` on an
unmatched brace.

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

Two things that have cost time here:

- `tmux send-keys` treats `;` as a command separator. Send it with
  `tmux send-keys -t zn -l '\;'` or it looks like a broken binding.
- `capture-pane` normalizes some output. For escape sequences, use
  `tmux pipe-pane` and read the raw bytes.

Manual checks worth running for anything touching save, watch or layout:

- Two terminals on the same day. Type in one, the other updates within a
  second. Then edit both at once and confirm the conflict behaves as the
  README describes, with no crash and no garbled file.
- A long line at a narrow width, to confirm wrapping and the cursor still
  agree.
- Light theme and dark theme, and a theme switch while the app is running.

## Pull requests

- One change per pull request. Do not bundle adjacent cleanup.
- Say what you verified and what it showed, not that you tested it.
- Commit subjects are imperative and plain: `Add screen motions, scroll
  positioning, and the vim staples`.
- Comments explain intent, trade-offs and constraints. Keep them to two lines,
  four for a doc comment. If a comment needs more, the code is the problem.
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

Deferred rather than rejected, and worth an issue first: markdown rendered
with glamour in normal mode, search across days, and rolling unchecked boxes
forward to the next day.

Open an issue before a large change. A pull request that gets rejected on
scope is a waste of your evening.
