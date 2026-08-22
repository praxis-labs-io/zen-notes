# zen-notes

One notepad per day. Run `zen-notes` from anywhere and you land in today's
note, already open. Two terminals on the same day stay in sync.

Markdown, syntax highlighted as you type, edited with vim motions. Images
render inline where your terminal supports it. Nothing else.

## Install

```sh
curl -fsSL https://raw.githubusercontent.com/praxis-labs-io/zen-notes/main/install.sh | sh
```

Downloads the binary for macOS or Linux, on arm64 or amd64, and needs nothing
else. Windows takes the `.zip` off the
[releases page](https://github.com/praxis-labs-io/zen-notes/releases), the
installer being a POSIX script. On anything else, or to build it yourself:

```sh
go install github.com/praxis-labs-io/zen-notes@latest
```

That one needs Go 1.26 or later. [Install](docs/install.md) covers where the
binary lands, where your notes end up, and upgrading.

## A first note

```sh
zen-notes
```

That is the whole of it. You are in today's note in normal mode, and vim motions
work. There is no save key: the buffer reaches disk within half a second of an
edit. `[` and `]` walk to yesterday and tomorrow, `\` comes back to today, and
`?` lists every binding without leaving the app.

## Documentation

- [Guide](docs/guide.md) — where notes live and how they sync, when an image
  draws, and what the screen is made of
- [Keys](docs/keys.md) — every binding, and what insert mode does with Markdown
- [Install](docs/install.md) — requirements, the notes directory, upgrading
- [Contributing](docs/CONTRIBUTING.md) — the layout, the test conventions, and
  what is deliberately out of scope

## License

MIT. See [LICENSE](LICENSE).

Built on [Bubble Tea](https://github.com/charmbracelet/bubbletea) and
[Lipgloss](https://github.com/charmbracelet/lipgloss) (MIT),
[fsnotify](https://github.com/fsnotify/fsnotify) (BSD-3-Clause),
[go-colorful](https://github.com/lucasb-eyer/go-colorful) and
[go-runewidth](https://github.com/mattn/go-runewidth) (MIT).
