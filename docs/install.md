# Install

## Requirements

- **Go 1.26 or later.** `go install` builds from source, and there is no other
  runtime dependency.
- **A terminal that speaks the kitty graphics protocol**, only if you want
  images to draw. Ghostty and kitty do. Everywhere else the reference stays
  text and nothing else changes.

## Install

```sh
go install github.com/praxis-labs-io/zen-notes@latest
```

That drops the binary in `$GOBIN`, or `$GOPATH/bin` if that is unset. To put it
somewhere else:

```sh
GOBIN=$HOME/.local/bin go install github.com/praxis-labs-io/zen-notes@latest
```

Then run it from anywhere:

```sh
zen-notes
```

`go install ...@latest` resolves to the newest tagged version, so what you get
is the last release rather than the tip of `main`.

## Where notes live

`$ZEN_NOTES_DIR`, or `~/.zen-notes` if that is unset. One flag, `-dir`,
overrides it for a single run:

```sh
zen-notes -dir ~/work-notes
```

[The guide](guide.md) covers what happens in that directory: one file per day,
how two terminals stay in sync, and what happens at midnight.

## Upgrading

Re-run the install command. It fetches the newest tag.

Nothing checks for updates and nothing phones home.
