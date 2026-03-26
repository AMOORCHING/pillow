package interrupt

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/pillow-sh/pillow/internal/bus"
)

// AccelClient connects to pillowsensord and emits slap interrupts.
type AccelClient struct {
	socketPath string
}

// NewAccelClient creates a client for the sensor daemon.
func NewAccelClient(socketPath string) *AccelClient {
	if socketPath == "" {
		socketPath = SensorSocket
	}
	return &AccelClient{socketPath: socketPath}
}

// Run connects to the sensor daemon and emits interrupts.
// It blocks until the context is cancelled.
func (a *AccelClient) Run(ctx context.Context, interrupts chan<- bus.Interrupt) error {
	conn, err := net.DialTimeout("unix", a.socketPath, 2*time.Second)
	if err != nil {
		return fmt.Errorf("connecting to sensor daemon at %s: %w — run with --no-slap or start pillowsensord", a.socketPath, err)
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
			log.Printf("slap detected (magnitude: %.2f)", evt.Magnitude)
			select {
			case interrupts <- bus.Interrupt{
				Type:      bus.InterruptSlap,
				Timestamp: evt.Timestamp,
				Amplitude: evt.Magnitude,
			}:
			case <-ctx.Done():
				return nil
			}
		}
	}

	return scanner.Err()
}

// SensordRunning checks if the sensor daemon is accessible.
func SensordRunning(socketPath string) bool {
	if socketPath == "" {
		socketPath = SensorSocket
	}
	conn, err := net.DialTimeout("unix", socketPath, 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
