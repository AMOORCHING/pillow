package interrupt

import "time"

// SensorSocket is the default Unix domain socket path for the sensor daemon.
const SensorSocket = "/tmp/pillow.sock"

// SlapEvent is sent from pillowsensord to pillow clients over the socket.
type SlapEvent struct {
	Type      string    `json:"type"`      // always "slap"
	Magnitude float64   `json:"magnitude"` // acceleration magnitude
	Timestamp time.Time `json:"timestamp"`
}

// SensordConfig is sent from pillow to pillowsensord to adjust settings.
type SensordConfig struct {
	Sensitivity float64 `json:"sensitivity"`
	CooldownMs  int     `json:"cooldown_ms"`
}
