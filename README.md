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

Press `?` for the binding list without leaving the app.

Modes: normal, insert, visual, visual-line, visual-block, and a `:` line.

| | |
|---|---|
| `h j k l` | move |
| arrows | stand in for `hjkl`, counts and operators included |
| `w W b B e E` | by word |
| `0 ^ $` | line start, first non-blank, line end |
| `gg G`, `{ }` | buffer ends, paragraphs |
| `f F t T` | find on the line |
| `; ,` | repeat that find, forward and back |
| `/` | search, `n` and `N` to walk the matches |
| `%` | matching bracket |
| `H M L` | top, middle, bottom of the window |
| `zt zz zb` | scroll the cursor's line to top, centre, bottom |
| `ctrl+d ctrl+u` | half page |
| `i a I A o O` | insert |
| `d c y` + motion, `dd cc yy` | operators |
| `iw aw i" a( ip` | text objects, as in `ciw` |
| `x X D C Y p P` | edit |
| `r s S` | replace a rune, substitute rune or line |
| `>> <<`, `>` + motion | indent |
| `~`, `gU gu g~` + motion | case |
| `gv` | reselect the last visual range |
| `J` | join lines |
| `v V ctrl+v` | visual, line, block |
| `> < ~ u U` | indent and case, in visual |
| `o` | swap ends of a selection |
| `I A` in visual-block | insert down the whole block |
| `u`, `ctrl+r` | undo, redo |
| `[` `]` `\` | previous day, next day, back to today |
| `?` | binding list |
| `:q` `:w` `:wq`, `ZZ`, `ctrl+c` | quit and save |

Counts work: `3j`, `d2w`, `2dd`.

Search is incremental: the cursor and the highlight follow along as you type,
and `esc` puts the cursor back where you started. It matches a plain substring
rather than a regular expression, and ignores case unless the pattern carries
a capital. Matches stay lit until `esc`. There is
no `?` for a backward search, because `?` opens the binding list; `N` walks
backward instead.

Not in yet: `.` repeat, named registers, marks, macros, and the jumplist.

## Looks

No borders. The note, then a status line: the mode bottom left, colored per
mode, and the filename hard right. Line numbers are hybrid, absolute on the
cursor's line and relative everywhere else, all in one column, in a gutter
that is always reserved so text never shifts.

No color is hardcoded. Text, gutter, status bar and help all use ANSI slots
0 to 15, so they come from your terminal theme. The caret is the real
terminal cursor, in your cursor color: a block in normal, a bar in insert.

The selection and the yank flash are the two computed colors. Both need a
background a step off your own, so zen-notes asks the terminal for its
background and shifts the lightness while keeping the hue, which keeps them
inside your theme. A step near white reads stronger than the same step near
black, so light themes get a smaller one. The background is asked for again
whenever the window regains focus, so switching theme mid-session is picked
up. If the terminal will not answer, and some multiplexers will not, it falls
back to a neutral grey and assumes dark.

A yank changes nothing on screen, so what it took lights up for a moment and
the status bar says how much.

## Development

```
go test ./...
go vet ./...
```
