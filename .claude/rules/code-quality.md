# Code Quality (zen-notes)

Go and Bubble Tea specifics for this repo. The principles live in the global
rules, which load automatically; this file only adds what the stack demands.
Don't restate the global rules here.

## Naming

- **Packages**: short, lowercase, no underscores (`note`, `editor`, `app`). One package per directory.
- **Files**: one lowercase word where the job has one (`buffer.go`, `motion.go`, `wrap.go`). Tests are `foo_test.go` beside `foo.go`.
- **Identifiers**: exported PascalCase, unexported camelCase. No stutter (`note.Store`, not `note.NoteStore`).
- **Constants**: Go style (`MixedCaps`), not SCREAMING_SNAKE.

## Errors

- Wrap with `%w` and context: `fmt.Errorf("write temp note: %w", err)`.
- An error you mean to drop is spelled `_ = f.Close()`, and the reason goes in a comment when it isn't obvious. Never a lint suppression: the violation is the signal to fix the code, not to annotate around it.

## Bubble Tea

- **The watcher goroutine never touches the model.** It owns its channel, and `waitForChange` reads that channel inside a `tea.Cmd` so the result reaches `Update` as a message. That is the whole reason `-race` stays quiet with a live filesystem watch, and it is the first thing to check if it stops.
- One message type per outcome, named for what happened (`fileChangedMsg`, `yankFlashDoneMsg`). No shared bag-of-fields message reused across call sites.
- **`Render` is the one thing `View` calls that writes.** It caches the wrapped rows, the cursor's row and the scroll top, because the caret position has to come from the same pass that wrapped the text. Nothing else in `View` mutates, and nothing in `View` fetches or starts anything.

## Tests

- Tests ship in the same change as the logic, never a follow-up.
- Table-driven for anything with more than two cases.

## File size

Keep files focused. A file too big to review in one sitting is doing too much;
split it before it gets there rather than after.
