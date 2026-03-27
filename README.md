# pillow

Voice-narrated agentic coding supervision with physical interrupts.

A background daemon that supervises agentic coding tools (starting with Claude Code), providing voice narration of critical events, irreversibility warnings, drift detection, and slap-to-interrupt negotiation.

## Install

### Homebrew (recommended)

```bash
brew install AMOORCHING/pillow/pillow-cli
```

This installs both `pillow` and `pillowsensord`.

### Go

```bash
go install github.com/AMOORCHING/pillow/cmd/pillow@latest
go install github.com/AMOORCHING/pillow/cmd/pillowsensord@latest  # optional, for slap detection
```

### Build from source

```bash
git clone https://github.com/AMOORCHING/pillow
cd pillow
make build    # also: make install, make test, make vet, make clean
```

## Quick Start

```bash
# 1. Run the setup wizard
pillow setup

# 2. Install the Claude Code plugin
bash plugin/install.sh

# 3. Start the daemon
pillow

# 4. Use Claude Code normally — pillow watches and narrates in the background
claude "refactor the auth middleware to use JWT"
```

## Architecture

pillow runs as a headless background daemon. Claude Code communicates with it via shell-script hooks over a Unix socket HTTP API. There is no TUI — silence is the default, and pillow only speaks up when something important happens.

```
Claude Code ──hooks──> pillow daemon ──> TTS
                           │
                  ┌────────┼────────┐
                  │        │        │
              classify   drift   summarize
              (heuristic) (Haiku)  (Haiku)
```

### What triggers narration

- **Irreversibility warnings** — dangerous operations like `rm -rf`, migrations, `.env` edits, destructive SQL
- **Drift detection** — agent veering off-track from its stated goal
- **Session end** — cost summary and recap

Everything else is silent.

## Usage

### Daemon

```bash
pillow                          # start the daemon (foreground)
pillow --socket-path /tmp/p.sock  # custom socket path
pillow --verbose                # verbose logging
```

### Commands

```bash
pillow setup                    # interactive setup wizard
pillow config                   # print current config
pillow config edit              # open config in $EDITOR
pillow status                   # daemon status and session info
pillow recap                    # request retrospective summary
pillow history                  # print recent interrupt events
pillow sensord start            # start sensor daemon (requires sudo)
pillow sensord stop             # stop sensor daemon
pillow sensord status           # check if sensor daemon is running
```

## Plugin (Claude Code Integration)

pillow integrates with Claude Code via [hooks](https://docs.anthropic.com/en/docs/claude-code/hooks). The plugin consists of shell scripts that send events to the pillow daemon over its Unix socket.

### Hooks

| Hook | What it does |
|------|-------------|
| `PreToolUse` | Sends tool call to daemon for classification, polls for slap interrupts, can block dangerous operations |
| `PostToolUse` | Logs completed tool calls (fire-and-forget) |
| `SessionStart` | Signals session start with goal text |
| `SessionEnd` | Triggers final summary and cost report |

### Install the plugin

```bash
bash plugin/install.sh
```

This copies hook scripts to `~/.config/pillow/hooks/` and merges hook configuration into Claude Code's `~/.claude/settings.json`.

## Irreversibility Classification

pillow classifies every tool call by danger level:

| Level | Action | Examples |
|-------|--------|---------|
| `block` | Narrates warning, can halt agent | `rm -rf`, `DROP TABLE`, migration files |
| `warn` | Narrates note | `.env` edits, `chmod`, `Dockerfile` changes |
| `none` | Silent | Regular reads, writes, searches |

## Drift Detection

When an Anthropic API key is configured, pillow periodically checks whether the agent is staying on track using a lightweight Haiku LLM call. Checks trigger:

- Every N tool calls (default: 10)
- After a pause longer than a threshold (default: 2s)
- With a cooldown to avoid over-checking (default: 30s)

If drift is detected, pillow narrates the reason aloud.

## Slap Detection

Slap your MacBook to interrupt the agent. Requires Apple Silicon and the sensor daemon.

```bash
pillow sensord start             # start accelerometer daemon (prompts for sudo)
pillow                           # daemon connects to sensord automatically
```

When a slap is detected, the PreToolUse hook polls it from the daemon and can block the next tool call, triggering a negotiation flow.

The sensor daemon (`pillowsensord`) runs as root to access the accelerometer. It communicates with the main pillow daemon over a separate Unix socket at `/tmp/pillowsensord.sock`.

## Privacy Modes

| Mode | Summarizer | TTS | Drift | What leaves your machine |
|------|-----------|-----|-------|--------------------------|
| `cloud` | Anthropic Haiku | Cartesia | Yes | Agent summaries, narration text, drift checks |
| `hybrid` | Anthropic Haiku | Local (piper/say) | Yes | Agent summaries, drift checks |
| `local` | Templates | Local (piper/say) | No | Nothing |

TTS never sees your source code — only summarized narration text.

## Configuration

Config lives at `~/.config/pillow/config.toml`. Run `pillow setup` to generate it interactively, or `pillow config edit` to edit manually.

```toml
privacy_mode = "cloud"             # "cloud" | "hybrid" | "local"

# API keys
anthropic_api_key = "sk-ant-..."
cartesia_api_key = "..."

# Voice
voice = ""                         # Cartesia voice ID (blank for default)
cartesia_model = "sonic-3"

# Drift detection
drift_check_interval = 10          # check every N tool calls
drift_pause_ms = 2000              # or on pause > this duration
drift_cooldown_s = 30              # suppress re-check after drift narration

# Summarization
summary_interval = 30              # compress every N events

# Slap detection
slap_threshold = 2.5               # g-force (0 to disable)
slap_debounce_ms = 1000
slap_stale_ms = 5000
slap_sound = "chime"               # "chime" | "none"

# IPC
socket_path = "/tmp/pillow.sock"
sensord_socket_path = "/tmp/pillowsensord.sock"

# Narration
narration_stale_ms = 3000          # drop narration items older than this
mute_while_typing_ms = 500         # suppress audio if user typed within this window
```

Environment variables `CARTESIA_API_KEY` and `ANTHROPIC_API_KEY` override config file values.

## Cost Tracking

pillow tracks its own API costs (TTS, summarizer, and drift checks) and reports them at session end:

```
Session complete. TTS: ~$0.008 (1247 chars), LLM: ~$0.003 (2891 in/412 out), Drift: ~$0.001 (500 in/50 out), Slaps: 3
```

Use `pillow status` to check costs mid-session, or `pillow recap` for the current rolling summary.

## IPC API

The daemon exposes an HTTP API over its Unix socket:

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/event` | POST | Submit a tool use event for classification |
| `/slap` | GET | Poll for buffered slap events |
| `/narrate` | POST | Immediately narrate text |
| `/summary` | GET | Get current rolling summary |
| `/session/start` | POST | Start a new session |
| `/session/end` | POST | End session, get cost + summary |
| `/status` | GET | Daemon status and session info |

## License

MIT
