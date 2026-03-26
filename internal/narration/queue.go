package narration

import (
	"sync"
	"time"
)

// Priority levels for narration items.
const (
	PriorityLow    = 0 // thinking, status updates
	PriorityNormal = 1 // tool use, file edits
	PriorityHigh   = 2 // errors, interrupts, completion
)

// Item is a single narration to be spoken.
type Item struct {
	Text      string
	Priority  int
	CreatedAt time.Time
}

// Queue is a priority-based narration queue with staleness eviction.
type Queue struct {
	mu             sync.Mutex
	items          []Item
	staleThreshold time.Duration
}

// NewQueue creates a narration queue with the given staleness threshold.
func NewQueue(staleThreshold time.Duration) *Queue {
	return &Queue{
		staleThreshold: staleThreshold,
	}
}

// Push adds a narration item to the queue.
func (q *Queue) Push(text string, priority int) {
	q.mu.Lock()
	defer q.mu.Unlock()

	item := Item{
		Text:      text,
		Priority:  priority,
		CreatedAt: time.Now(),
	}

	// High priority items flush the queue
	if priority >= PriorityHigh {
		q.items = []Item{item}
		return
	}

	q.items = append(q.items, item)
}

// Pop returns the next narration item, skipping stale ones.
// Returns empty string if the queue is empty.
func (q *Queue) Pop() (Item, bool) {
	q.mu.Lock()
	defer q.mu.Unlock()

	now := time.Now()
	for len(q.items) > 0 {
		item := q.items[0]
		q.items = q.items[1:]

		// Skip stale items (but never skip high priority)
		if item.Priority < PriorityHigh && now.Sub(item.CreatedAt) > q.staleThreshold {
			continue
		}

		return item, true
	}
	return Item{}, false
}

// Flush removes all items from the queue.
func (q *Queue) Flush() {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.items = nil
}

// Len returns the number of items in the queue.
func (q *Queue) Len() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return len(q.items)
}
