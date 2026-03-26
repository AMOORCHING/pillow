# pillow

Voice-narrated agentic coding with physical interrupts.

A CLI wrapper that adds real-time voice narration to agentic coding tools (starting with Claude Code) and lets you slap your MacBook to interrupt or redirect the agent.

## Install

### Homebrew (recommended)

```bash
brew tap AMOORCHING/pillow
brew install pillow
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
make build
```

## Quick Start

```bash
pillow claude "refactor the auth middleware to use JWT"
```

On first run, pillow walks you through a 30-second setup wizard to configure your API keys and preferences.

## Usage

```bash
pillow <agent> [prompt]          # run an agent with voice narration
pillow setup                     # interactive setup wizard
pillow config                    # open config in $EDITOR
pillow sensord status            # check if sensor daemon is running
```

### Flags

```
--quiet / --no-voice    Mute narration audio
--no-slap               Disable accelerometer, keyboard only
--privacy <mode>        Override privacy mode (cloud/hybrid/local)
```

## How It Works

pillow spawns Claude Code with `--output-format stream-json`, parses the event stream, summarizes what the agent is doing via a fast LLM (Haiku), and speaks the summary aloud via TTS (Cartesia Sonic or macOS `say`).

```
Agent output → Summarizer (LLM) → TTS → Speaker
                    ↑                      ↑
              Haiku or regex          Cartesia, piper, or say
```

### Interrupts

- **Slap your MacBook** — pauses the agent, plays "ow!", prompts for input
- **Ctrl+\\** — narrates current status
- **Ctrl+C** — kills everything (standard Unix)

Slap detection requires Apple Silicon and the sensor daemon (`sudo pillowsensord`). Without it, keyboard interrupts work on all platforms.

## Privacy Modes

| Mode | Summarizer | TTS | What leaves your machine |
|------|-----------|-----|--------------------------|
| `cloud` | Anthropic API (Haiku) | Cartesia API | Agent summaries + narration text |
| `hybrid` | Anthropic API (Haiku) | Local (piper/say) | Agent summaries only |
| `local` | Regex/templates | Local (piper/say) | Nothing |

TTS never sees your source code — only the summarized narration text.

## Configuration

Config lives at `~/.config/pillow/config.toml`. Run `pillow setup` to generate it interactively, or `pillow config` to edit manually.

```toml
[tts]
provider = "cartesia"         # cartesia, piper, say
cartesia_api_key = "sk-..."
cartesia_voice = ""           # voice UUID (uses default if empty)

[narration]
anthropic_api_key = "sk-..."
model = "claude-haiku-4-5-20251001"
stale_threshold_ms = 3000
batch_pause_ms = 500

[interrupt]
slap_enabled = true
sensitivity = 0.15
cooldown_ms = 750

[privacy]
mode = "cloud"

[cost]
show_live = true
show_summary = true
```

Environment variables `CARTESIA_API_KEY` and `ANTHROPIC_API_KEY` override config file values.

## Sensor Daemon

Slap detection requires accelerometer access, which needs root on macOS. pillow uses a split architecture:

- **pillowsensord** — tiny daemon that reads the accelerometer (runs as root)
- **pillow** — main process that does everything else (runs as your user)

They communicate over a Unix socket at `/tmp/pillow.sock`.

```bash
sudo pillowsensord &           # start sensor daemon
pillow claude "fix the bug"    # pillow connects automatically
```

## Cost Tracking

pillow tracks its own API costs (TTS + summarizer) and prints a session summary on exit:

```
pillow session summary
──────────────────────
  Duration:     4m 32s
  Slaps:        7
  Narrations:   12
  Cost breakdown:
    TTS:               $0.0080  (1247 chars)
    LLM (summarizer):  $0.0030  (2891 input / 412 output tokens)
    Pillow total:     ~$0.0110
```

## Tip

You can run `pillow claude` and use `/voice` inside Claude Code — speak your prompt via Claude Code's voice mode, then listen to pillow narrate the implementation back to you. Fully hands-free coding loop.

## License

MIT
