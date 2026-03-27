package history

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/AMOORCHING/pillow/internal/config"
)

// InterruptEvent records a user interruption and its outcome.
type InterruptEvent struct {
	Timestamp   time.Time `json:"timestamp"`
	SessionID   string    `json:"session_id"`
	AgentAction string    `json:"agent_action"` // tool name + file
	FilePath    string    `json:"file_path"`
	UserInput   string    `json:"user_input"`   // what user typed
	Outcome     string    `json:"outcome"`       // stop | continue | revert | redirect
}

// Store manages append-only JSONL history.
type Store struct {
	path string
}

// NewStore creates a history store at the default path.
func NewStore() *Store {
	return &Store{path: config.HistoryPath()}
}

// Append writes an interrupt event to the JSONL file.
func (s *Store) Append(evt InterruptEvent) error {
	// Ensure directory exists
	dir := config.ConfigDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating history dir: %w", err)
	}

	f, err := os.OpenFile(s.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return fmt.Errorf("opening history file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	data = append(data, '\n')

	_, err = f.Write(data)
	return err
}

// Read returns all interrupt events from the history file.
func (s *Store) Read() ([]InterruptEvent, error) {
	data, err := os.ReadFile(s.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var events []InterruptEvent
	dec := json.NewDecoder(
		&byteReader{data: data},
	)
	for dec.More() {
		var evt InterruptEvent
		if err := dec.Decode(&evt); err != nil {
			continue
		}
		events = append(events, evt)
	}
	return events, nil
}

// ReadLast returns the last N events.
func (s *Store) ReadLast(n int) ([]InterruptEvent, error) {
	all, err := s.Read()
	if err != nil {
		return nil, err
	}
	if len(all) <= n {
		return all, nil
	}
	return all[len(all)-n:], nil
}

type byteReader struct {
	data []byte
	pos  int
}

func (r *byteReader) Read(p []byte) (n int, err error) {
	if r.pos >= len(r.data) {
		return 0, fmt.Errorf("EOF")
	}
	n = copy(p, r.data[r.pos:])
	r.pos += n
	return n, nil
}
