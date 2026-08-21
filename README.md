# zen-notes

One notepad per day. Run `zen-notes` from anywhere and you land in today's
note, already open. Two terminals on the same day stay in sync.

Markdown, syntax highlighted as you type, edited with vim motions. Images
render inline where your terminal supports it. Nothing else.

## Install

```
go install github.com/praxis-labs-io/zen-notes@latest
```

That drops the binary in `$GOBIN`, or `$GOPATH/bin` if that is unset. To put
it somewhere else:

```
GOBIN=$HOME/.local/bin go install github.com/praxis-labs-io/zen-notes@latest
```

Then run it from anywhere:

```
zen-notes
```

Go 1.26 or later. No other runtime dependency.

## Storage and sync

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

There is one flag, `-dir`, which overrides `$ZEN_NOTES_DIR` for a single run.

## Images

A line holding nothing but an image reference draws the image under it:

```
![a sketch](sketch.png)
```

The line stays as you typed it, and the picture sits below. Paths are
relative to your notes directory, or absolute, or start with `~`. Nothing
is fetched over the network.

This needs a terminal that speaks the kitty graphics protocol, which
Ghostty and kitty do. zen-notes asks the terminal how big a cell is, and
draws images only if it answers. Everywhere else the line stays text, and
nothing else changes. tmux does not pass these images through.

An image is scaled to fit the width of your note and half the window, so
it never buries what you wrote around it.

## Keys

Press `?` for the binding list without leaving the app.

Modes: normal, insert, visual, visual-line, visual-block, and a `:` line.

| | |
|---|---|
| `h j k l` | move by logical line |
| `gj gk` | move by wrapped display row |
| arrows | stand in for `hjkl`, counts and operators included |
| `w W b B e E` | by word |
| `0 ^ $` | line start, first non-blank, line end |
| `gg G`, `{ }` | buffer ends, paragraphs |
| `f F t T` | find on the line |
| `; ,` | repeat that find, forward and back |
| `/` | search, `n` and `N` to walk the matches |
| `%` | matching bracket |
| `gx` | open the Markdown link under the cursor |
| `H M L` | top, middle, bottom of the window |
| `zt zz zb` | scroll the cursor's line to top, centre, bottom |
| `ctrl+d ctrl+u` | half page |
| `i a I A o O` | insert |
| `d c y` + motion, `dd cc yy` | operators |
| `iw aw i" a( ip` | text objects, as in `ciw` |
| `x X D C Y p P` | edit |
| `cmd+c`, terminal paste | copy a visual selection, paste the system clipboard |
| `r s S` | replace a rune, substitute rune or line |
| `tab shift+tab` in a list | nest or unnest an item and its children |
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

The Vim register and system clipboard move together. Yanks, deletes, changes
and substitutes update both. `p` and `P` reuse that register. Cmd+C copies an
active visual selection when the terminal forwards Command keys. The terminal's
paste shortcut imports external clipboard text, then pastes it in the active
insert, normal or visual mode.

Insert mode follows the Markdown around the cursor. Enter after a heading leaves
a blank line for body text. Starting an unordered, ordered or task list, using
either `- [ ]` or a bare `[ ]`, leaves a blank line after preceding prose. Enter
continues its marker and indentation; a checked task continues as an unchecked
one. Tab and Shift-Tab
nest or unnest an item and its children by two spaces, once it has a preceding
sibling to nest beneath. Enter or Backspace on an empty nested item moves it out
one level; at the top level, either exits with a blank line below.

Search is incremental: the cursor and the highlight follow along as you type,
and `esc` puts the cursor back where you started. It matches a plain substring
rather than a regular expression, and ignores case unless the pattern carries
a capital. Matches stay lit until `esc`. There is no `?` for a backward
search, because `?` opens the binding list; `N` walks backward instead.

Not in yet: `.` repeat, named registers, marks, macros, and the jumplist.

## Looks

No borders. The note, then a status line: the mode bottom left, colored per
mode, and the filename hard right. Line numbers are hybrid, absolute on the
cursor's line and relative everywhere else, all in one column, in a gutter
that is always reserved so text never shifts.

The cursor's line carries a band across the full width, gutter included, in
normal and insert mode. It goes out in visual mode, where the selection is
the thing to look at. A wrapped line is banded on every row it takes, and
an image's rows take no band, so the picture is not framed by one.

No color is hardcoded. Text, gutter, status bar and help all use ANSI slots
0 to 15, so they come from your terminal theme. The caret is the real
terminal cursor, in your cursor color: a block in normal, a bar in insert.

The cursor line, the selection, the yank flash and the search highlight are
the four computed colors. Each needs a background a step off your own, so
zen-notes asks the terminal for its background and shifts the lightness while
keeping the hue, which keeps them inside your theme. A step near white reads
stronger than the same step near black, so light themes get a smaller one,
and the cursor line takes the smallest step of the four. The background is
asked for again whenever the window regains focus, so switching theme
mid-session is picked up. If the terminal will not answer, and some
multiplexers will not, it falls back to a neutral grey and assumes dark.

A yank changes nothing on screen, so what it took lights up for a moment and
the status bar says how much.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for the layout, the test conventions,
and what is deliberately out of scope.

## License

MIT. See [LICENSE](LICENSE).

Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lipgloss](https://github.com/charmbracelet/lipgloss) (MIT),
[fsnotify](https://github.com/fsnotify/fsnotify) (BSD-3-Clause),
[go-colorful](https://github.com/lucasb-eyer/go-colorful) and
[go-runewidth](https://github.com/mattn/go-runewidth) (MIT).
