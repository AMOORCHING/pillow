package narration

// The narration engine in the new architecture is simplified.
// The daemon handler (internal/daemon) drives narration directly:
// - Events are classified by internal/classify
// - The silence-first filter (internal/narration/filter.go) decides what to narrate
// - Narration text is pushed to the Queue
// - The daemon's speakNext() pops from the queue and calls TTS
//
// The rolling summarizer runs on a cadence controlled by the daemon.
// The old channel-based Engine is no longer needed — the daemon is the engine.
