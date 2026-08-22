# Keys

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

The concepts behind them are in [the guide](guide.md), and how to install
zen-notes in [install](install.md).
