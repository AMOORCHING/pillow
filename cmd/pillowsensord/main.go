package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/pillow-sh/pillow/internal/interrupt"
)

func main() {
	if os.Geteuid() != 0 {
		fmt.Fprintln(os.Stderr, "pillowsensord requires root privileges for accelerometer access.")
		fmt.Fprintln(os.Stderr, "Run with: sudo pillowsensord")
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle shutdown signals
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		log.Println("shutting down...")
		cancel()
	}()

	// Remove stale socket
	os.Remove(interrupt.SensorSocket)

	// Listen on Unix domain socket
	listener, err := net.Listen("unix", interrupt.SensorSocket)
	if err != nil {
		log.Fatalf("failed to listen on %s: %v", interrupt.SensorSocket, err)
	}
	defer listener.Close()
	defer os.Remove(interrupt.SensorSocket)

	// Make socket accessible to non-root users
	if err := os.Chmod(interrupt.SensorSocket, 0666); err != nil {
		log.Printf("warning: couldn't chmod socket: %v", err)
	}

	log.Printf("pillowsensord listening on %s", interrupt.SensorSocket)

	// Track connected clients
	var (
		clients   []net.Conn
		clientsMu sync.Mutex
	)

	// Accept connections
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				if ctx.Err() != nil {
					return
				}
				log.Printf("accept error: %v", err)
				continue
			}
			clientsMu.Lock()
			clients = append(clients, conn)
			clientsMu.Unlock()
			log.Printf("client connected (%d total)", len(clients))
		}
	}()

	// Accelerometer polling loop
	// NOTE: This uses a stub implementation. When building for real hardware,
	// replace with taigrr/apple-silicon-accelerometer integration.
	log.Println("accelerometer monitoring active (stub mode — replace with real sensor)")
	runAccelerometerLoop(ctx, func(magnitude float64) {
		evt := interrupt.SlapEvent{
			Type:      "slap",
			Magnitude: magnitude,
			Timestamp: time.Now(),
		}
		data, err := json.Marshal(evt)
		if err != nil {
			return
		}
		data = append(data, '\n')

		clientsMu.Lock()
		defer clientsMu.Unlock()

		// Write to all clients, removing dead ones
		alive := clients[:0]
		for _, conn := range clients {
			conn.SetWriteDeadline(time.Now().Add(1 * time.Second))
			if _, err := conn.Write(data); err != nil {
				conn.Close()
				log.Println("client disconnected")
				continue
			}
			alive = append(alive, conn)
		}
		clients = alive
	})

	<-ctx.Done()
}

// runAccelerometerLoop is a stub that will be replaced with real sensor integration.
// For now, it just sleeps. The real implementation will use:
//   import "github.com/taigrr/apple-silicon-accelerometer/accel"
// and detect vibrations using STA/LTA algorithm.
func runAccelerometerLoop(ctx context.Context, onSlap func(magnitude float64)) {
	// Stub: In production, this polls the accelerometer via IOKit HID
	// and runs the vibration detection pipeline from spank.
	// For development without hardware, this is a no-op.
	<-ctx.Done()
}
