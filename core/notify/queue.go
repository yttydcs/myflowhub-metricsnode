package notify

import "sync"

const DefaultQueueCapacity = 128

type Queue struct {
	mu      sync.Mutex
	cap     int
	items   []Event
	dropped uint64
}

func NewQueue(capacity int) *Queue {
	if capacity <= 0 {
		capacity = DefaultQueueCapacity
	}
	return &Queue{cap: capacity}
}

func (q *Queue) Enqueue(evt Event) {
	if q == nil {
		return
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.cap <= 0 {
		q.dropped++
		return
	}
	if len(q.items) >= q.cap {
		copy(q.items, q.items[1:])
		q.items[len(q.items)-1] = evt
		q.dropped++
		return
	}
	q.items = append(q.items, evt)
}

func (q *Queue) DequeueAll() []Event {
	if q == nil {
		return nil
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.items) == 0 {
		return nil
	}
	out := make([]Event, len(q.items))
	copy(out, q.items)
	q.items = nil
	return out
}

func (q *Queue) Dropped() uint64 {
	if q == nil {
		return 0
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.dropped
}
