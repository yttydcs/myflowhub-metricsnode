package notify

import "testing"

func TestQueue_DequeueAllAndOverflow(t *testing.T) {
	q := NewQueue(2)
	q.Enqueue(Event{ID: "1", Title: "one"})
	q.Enqueue(Event{ID: "2", Title: "two"})
	q.Enqueue(Event{ID: "3", Title: "three"})

	got := q.DequeueAll()
	if len(got) != 2 {
		t.Fatalf("expected 2 events, got %d", len(got))
	}
	if got[0].ID != "2" || got[1].ID != "3" {
		t.Fatalf("expected oldest dropped, got %+v", got)
	}
	if q.Dropped() != 1 {
		t.Fatalf("expected dropped=1, got %d", q.Dropped())
	}
	if again := q.DequeueAll(); len(again) != 0 {
		t.Fatalf("expected empty queue, got %d", len(again))
	}
}
