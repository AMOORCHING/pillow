package ipc

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/AMOORCHING/pillow/internal/agent"
)

// EventHandler processes agent events and returns classification + narration decisions.
type EventHandler interface {
	HandleEvent(ctx context.Context, evt agent.AgentEvent) agent.EventResponse
	HandleSessionStart(ctx context.Context, req agent.SessionStartRequest)
	HandleSessionEnd(ctx context.Context, req agent.SessionEndRequest) agent.SessionEndResponse
	HandleNarrate(ctx context.Context, req agent.NarrateRequest)
	GetSummary() agent.SummaryResponse
	GetStatus() agent.StatusResponse
	PollSlap() *agent.SlapEvent
}

// Server is an HTTP-over-Unix-socket server for daemon IPC.
type Server struct {
	socketPath string
	handler    EventHandler
	listener   net.Listener
	httpServer *http.Server
	mu         sync.Mutex
}

// NewServer creates a new IPC server.
func NewServer(socketPath string, handler EventHandler) *Server {
	s := &Server{
		socketPath: socketPath,
		handler:    handler,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /event", s.handleEvent)
	mux.HandleFunc("GET /slap", s.handleSlap)
	mux.HandleFunc("POST /narrate", s.handleNarrate)
	mux.HandleFunc("GET /summary", s.handleSummary)
	mux.HandleFunc("POST /session/start", s.handleSessionStart)
	mux.HandleFunc("POST /session/end", s.handleSessionEnd)
	mux.HandleFunc("GET /status", s.handleStatus)

	s.httpServer = &http.Server{Handler: mux}
	return s
}

// Start begins listening on the Unix socket. It blocks until the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	// Remove stale socket
	os.Remove(s.socketPath)

	listener, err := net.Listen("unix", s.socketPath)
	if err != nil {
		return fmt.Errorf("listening on %s: %w", s.socketPath, err)
	}

	// Make socket accessible to non-root users
	if err := os.Chmod(s.socketPath, 0666); err != nil {
		log.Printf("[pillow] warning: couldn't chmod socket: %v", err)
	}

	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	log.Printf("[pillow] daemon listening on %s", s.socketPath)

	// Shutdown on context cancellation
	go func() {
		<-ctx.Done()
		s.httpServer.Shutdown(context.Background())
		os.Remove(s.socketPath)
	}()

	err = s.httpServer.Serve(listener)
	if err == http.ErrServerClosed {
		return nil
	}
	return err
}

func (s *Server) handleEvent(w http.ResponseWriter, r *http.Request) {
	var evt agent.AgentEvent
	if err := json.NewDecoder(r.Body).Decode(&evt); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if evt.Timestamp.IsZero() {
		evt.Timestamp = time.Now()
	}

	resp := s.handler.HandleEvent(r.Context(), evt)
	writeJSON(w, resp)
}

func (s *Server) handleSlap(w http.ResponseWriter, r *http.Request) {
	evt := s.handler.PollSlap()
	if evt == nil {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	writeJSON(w, evt)
}

func (s *Server) handleNarrate(w http.ResponseWriter, r *http.Request) {
	var req agent.NarrateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.handler.HandleNarrate(r.Context(), req)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSummary(w http.ResponseWriter, r *http.Request) {
	resp := s.handler.GetSummary()
	writeJSON(w, resp)
}

func (s *Server) handleSessionStart(w http.ResponseWriter, r *http.Request) {
	var req agent.SessionStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	s.handler.HandleSessionStart(r.Context(), req)
	w.WriteHeader(http.StatusOK)
}

func (s *Server) handleSessionEnd(w http.ResponseWriter, r *http.Request) {
	var req agent.SessionEndRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	resp := s.handler.HandleSessionEnd(r.Context(), req)
	writeJSON(w, resp)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	resp := s.handler.GetStatus()
	writeJSON(w, resp)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
