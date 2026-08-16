# zen-notes

One notepad per day. Run `zen-notes` from anywhere and you land in today's
note, already open. Two terminals on the same day stay in sync.

Markdown, syntax highlighted as you type, edited with vim motions. Nothing
else.

## Install

```
GOBIN=$HOME/.local/bin go install .
```

Then run it from anywhere:

```
zen-notes
```

## Storage

Notes live in `$ZEN_NOTES_DIR`, or `~/.zen-notes` if that is unset. One file
per day, named `2026-08-16.md`. Point the variable at an iCloud or Dropbox
folder and cross-machine sync is that service's job.

There is no save key. The buffer reaches disk within half a second of an edit,
written to a temp file and renamed over the target so a reader never sees a
partial note. When another window writes the note you have open, it reloads.
If you have unsaved edits at that moment, yours are kept and the status bar
says so. Last write wins, nothing is merged.

Past midnight the app moves to the new day on its own, unless you have browsed
away from today.

## Keys

Modes: normal, insert, visual, visual-line, and a `:` line.

| | |
|---|---|
| `h j k l`, arrows | move |
| `w W b B e E` | by word |
| `0 ^ $` | line start, first non-blank, line end |
| `gg G`, `{ }` | buffer ends, paragraphs |
| `f F t T` | find on the line |
| `ctrl+d ctrl+u` | half page |
| `i a I A o O` | insert |
| `d c y` + motion, `dd cc yy` | operators |
| `x X D C Y p P` | edit |
| `v V` | visual, then `d c y` |
| `u`, `ctrl+r` | undo, redo |
| `[` `]` `\` | previous day, next day, back to today |
| `:q` `:w` `:wq`, `ZZ`, `ctrl+c` | quit and save |

Counts work: `3j`, `d2w`, `2dd`.

Not in yet: `.` repeat, named registers, macros, marks, `/` search, and
glamour rendering in normal mode.

## Development

```
go test ./...
go vet ./...
```
