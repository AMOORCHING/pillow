# pillow

Voice guardrails for coding agents.

`pillow` runs in the background while you use Claude Code. It stays quiet most of the time, and only speaks when something needs your attention:

- dangerous or irreversible operations
- possible goal drift
- session recap + cost summary

It can also support physical slap-to-interrupt on Apple Silicon Macs.

## 5-minute setup

### 1) Install

#### Homebrew (recommended)

```bash
brew install AMOORCHING/pillow/pillow-cli
```

#### Go

```bash
go install github.com/AMOORCHING/pillow/cmd/pillow@latest
go install github.com/AMOORCHING/pillow/cmd/pillowsensord@latest # optional, slap detection
```

#### Build from source

```bash
git clone https://github.com/AMOORCHING/pillow
cd pillow
make build
```

This produces `./bin/pillow` and `./bin/pillowsensord`. You can either run them
directly from `./bin/` or copy them somewhere in your `PATH`:

```bash
sudo cp bin/pillow bin/pillowsensord /usr/local/bin/
```

### 2) Run first-time setup

```bash
pillow setup          # if installed to PATH
./bin/pillow setup    # if running from source
```

This creates `~/.config/pillow/config.toml` and walks you through:

- Anthropic key (for cloud summaries + drift checks)
- Cartesia key (for cloud voice)
- privacy mode (`cloud`, `hybrid`, or `local`)
- optional slap detection

### 3) Install Claude Code hooks

```bash
bash plugin/install.sh
```

What this does:

- installs hook scripts to `~/.config/pillow/hooks/`
- installs `pillow-hook` into your PATH
- merges hook config into `~/.claude/settings.json` (when `jq` is installed)

If `jq` is missing, install it and run the installer again.

### 4) Start pillow daemon

In one terminal, start the main daemon:

```bash
pillow                # if installed to PATH
./bin/pillow          # if running from source
```

Keep this running.

If you want slap-to-interrupt (Apple Silicon only), start the sensor daemon in a
**second terminal** with `sudo` (it needs root for accelerometer access):

```bash
sudo pillowsensord          # if installed to PATH
sudo ./bin/pillowsensord    # if running from source
```

### 5) Use Claude Code as usual

```bash
claude "refactor auth middleware to use JWT"
```

If hooks are installed correctly, pillow will now monitor tool calls and narrate important events.

> **Note:** The hooks silently no-op when the daemon isn't running — you won't
> see errors in Claude Code if you forget to start pillow.

## Verify everything works

Run these checks after setup:

```bash
pillow status
pillow config
```

Expected behavior:

- `pillow status` says the daemon is running (when started)
- `pillow config` prints your saved config

## Most common commands

```bash
pillow                 # start daemon (foreground)
pillow --verbose       # verbose logs
pillow setup           # setup wizard
pillow config          # show current config
pillow config edit     # edit config in $EDITOR
pillow status          # daemon/session status
pillow recap           # get current session recap
pillow history         # recent interrupt history
```

### Slap detection (optional)

Requires Apple Silicon. The sensor daemon needs `sudo` for accelerometer access
and must run in its own terminal alongside the main daemon:

```bash
sudo pillowsensord          # if installed to PATH
sudo ./bin/pillowsensord    # if running from source
```

You can also manage it through the main CLI:

```bash
pillow sensord start    # starts pillowsensord in background (needs sudo)
pillow sensord status
pillow sensord stop
```

## Privacy modes

| Mode | Summarizer | TTS | Drift detection | Data leaving machine |
|------|-----------|-----|-----------------|----------------------|
| `cloud` | Anthropic Haiku | Cartesia | Yes | summaries, narration text, drift checks |
| `hybrid` | Anthropic Haiku | Local (`say`/`piper`) | Yes | summaries, drift checks |
| `local` | Template-based | Local (`say`/`piper`) | No | nothing |

Environment variables `ANTHROPIC_API_KEY` and `CARTESIA_API_KEY` override config file values.

## What pillow actually does

`pillow` is a daemon with a local Unix socket API. Claude Code hooks send tool events to it.

It classifies calls into:

- `block` (can interrupt and require negotiation)
- `warn` (narrated caution)
- `none` (silent)

By default it does not narrate every event; it only narrates high-signal moments.

## Troubleshooting

### `pillow status` says daemon is not running

Start it in another terminal:

```bash
pillow
```

### No narration while using Claude Code

Check:

1. `pillow` daemon is running
2. `bash plugin/install.sh` completed successfully
3. `~/.claude/settings.json` contains pillow hooks
4. `pillow-hook` is available in your `PATH`

### Installer says `jq` not found

Install `jq`, then run:

```bash
bash plugin/install.sh
```

### Slap detection does not work

- only supported on Apple Silicon Mac
- `pillowsensord` must be running as root — either `sudo pillowsensord` in a
  separate terminal, or `pillow sensord start` (which invokes `sudo` for you)
- slap threshold may be too high in config — check with `pillow config edit`

## Development

```bash
make build
make test
make vet
make clean
```

## License

MIT
