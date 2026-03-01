package runtime

import (
	"context"
	"sort"
	"strings"
	"sync"
)

type ControlAction struct {
	Metric string `json:"metric"`
	Value  string `json:"value"`
}

type actionQueue struct {
	mu     sync.Mutex
	notify chan struct{}
	latest map[string]string
}

func newActionQueue() *actionQueue {
	return &actionQueue{
		notify: make(chan struct{}, 1),
		latest: make(map[string]string),
	}
}

func (q *actionQueue) Enqueue(metric, value string) {
	if q == nil {
		return
	}
	metric = strings.TrimSpace(metric)
	value = strings.TrimSpace(value)
	if metric == "" || value == "" {
		return
	}

	q.mu.Lock()
	if q.latest == nil {
		q.latest = make(map[string]string)
	}
	q.latest[metric] = value
	q.mu.Unlock()

	select {
	case q.notify <- struct{}{}:
	default:
	}
}

func (q *actionQueue) DequeueAll() []ControlAction {
	if q == nil {
		return nil
	}

	q.mu.Lock()
	if len(q.latest) == 0 {
		q.mu.Unlock()
		return nil
	}
	actions := make([]ControlAction, 0, len(q.latest))
	for metric, value := range q.latest {
		actions = append(actions, ControlAction{Metric: metric, Value: value})
	}
	q.latest = make(map[string]string)
	q.mu.Unlock()

	sort.Slice(actions, func(i, j int) bool { return actions[i].Metric < actions[j].Metric })
	return actions
}

func (q *actionQueue) Wait(ctx context.Context) bool {
	if q == nil {
		return false
	}
	select {
	case <-ctx.Done():
		return false
	case <-q.notify:
		return true
	}
}

