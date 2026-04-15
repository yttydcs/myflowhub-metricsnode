package runtime

// 本文件承载 MetricsNode 应用层中与 `control_queue` 相关的逻辑。

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

// newActionQueue 创建只保留每个 metric 最新值的控制动作队列，避免重复积压旧指令。
func newActionQueue() *actionQueue {
	return &actionQueue{
		notify: make(chan struct{}, 1),
		latest: make(map[string]string),
	}
}

// Enqueue 以 metric 为键覆盖旧值，保证消费者拿到的是最新控制意图。
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

// DequeueAll 批量取走当前快照，并按 metric 排序让下游处理结果稳定可预测。
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

// Wait 阻塞到有新动作或上下文取消，用于控制 worker 在空闲时休眠等待。
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
