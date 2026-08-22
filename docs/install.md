# Install

## Requirements

- **Nothing**, for a released binary. Everything is pure Go and statically
  linked, so there is no libc to match and no runtime to install.
- **Go 1.26 or later**, only if you are building it yourself.
- **A terminal that speaks the kitty graphics protocol**, only if you want
  images to draw. Ghostty and kitty do. Everywhere else the reference stays
  text and nothing else changes.

Releases carry macOS and Linux on arm64 and amd64, and Windows on amd64.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/praxis-labs-io/zen-notes/main/install.sh | sh
```

It downloads the binary for your platform and puts it in `~/.local/bin`. It is a
POSIX script, so Windows takes the `.zip` off the
[releases page](https://github.com/praxis-labs-io/zen-notes/releases) instead.
On a platform no release carries, the script says so and points you at Go.

`INSTALL_DIR` overrides where it lands, and `VERSION` pins a release:

```sh
curl -fsSL https://raw.githubusercontent.com/praxis-labs-io/zen-notes/main/install.sh | INSTALL_DIR=/usr/local/bin sh
curl -fsSL https://raw.githubusercontent.com/praxis-labs-io/zen-notes/main/install.sh | VERSION=v0.2.0 sh
```

Every release carries a `checksums.txt` beside the archives if you want to
verify one before it runs.

### With Go

```sh
go install github.com/praxis-labs-io/zen-notes@latest
```

That drops the binary in `$GOBIN`, or `$GOPATH/bin` if that is unset. To put it
somewhere else:

```sh
GOBIN=$HOME/.local/bin go install github.com/praxis-labs-io/zen-notes@latest
```

`go install ...@latest` resolves to the newest tagged version, so what you get
is the last release rather than the tip of `main`. It reports `dev` rather than
a version, being a build and not a release.

Then run it from anywhere:

```sh
zen-notes
```

## Where notes live

`$ZEN_NOTES_DIR`, or `~/.zen-notes` if that is unset. One flag, `-dir`,
overrides it for a single run:

```sh
zen-notes -dir ~/work-notes
```

[The guide](guide.md) covers what happens in that directory: one file per day,
how two terminals stay in sync, and what happens at midnight.

## Upgrading

Re-run whichever install command you used. Both fetch the newest release.

Nothing checks for updates and nothing phones home. `zen-notes -version` says
what you are running, and reports `dev` on a source build: the version is
stamped in at link time, and only the release workflow stamps it.
