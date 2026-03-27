package interrupt

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/AMOORCHING/pillow/internal/agent"
)

// SlapCallback is called when a slap is detected.
type SlapCallback func(evt agent.SlapEvent)

// AccelClient connects to pillowsensord and forwards slap events.
type AccelClient struct {
	socketPath string
	onSlap     SlapCallback
}

// NewAccelClient creates a client for the sensor daemon.
func NewAccelClient(socketPath string, onSlap SlapCallback) *AccelClient {
	if socketPath == "" {
		socketPath = DefaultSensordSocket
	}
	return &AccelClient{socketPath: socketPath, onSlap: onSlap}
}

// Run connects to the sensor daemon and forwards slap events.
// It blocks until the context is cancelled.
func (a *AccelClient) Run(ctx context.Context) error {
	conn, err := net.DialTimeout("unix", a.socketPath, 2*time.Second)
	if err != nil {
		return fmt.Errorf("connecting to sensord at %s: %w", a.socketPath, err)
	}
	defer conn.Close()

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		var evt SlapEvent
		if err := json.Unmarshal(scanner.Bytes(), &evt); err != nil {
			continue
		}

		if evt.Type == "slap" {
			log.Printf("[pillow] slap detected (magnitude: %.2f)", evt.Magnitude)
			a.onSlap(agent.SlapEvent{
				Timestamp: evt.Timestamp,
				Force:     evt.Magnitude,
			})
		}
	}

	return scanner.Err()
}

// SensordRunning checks if the sensor daemon is accessible.
func SensordRunning(socketPath string) bool {
	if socketPath == "" {
		socketPath = DefaultSensordSocket
	}
	conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
