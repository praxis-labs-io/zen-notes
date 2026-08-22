# Guide

One notepad per day. Where the notes live, when a picture draws, and what the
screen is made of.

The keymap is in [keys](keys.md), and how to install it in [install](install.md).

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
