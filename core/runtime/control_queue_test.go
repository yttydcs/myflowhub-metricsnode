package runtime

// 本文件覆盖 MetricsNode 应用层中与 `control_queue` 相关的行为。

import "testing"

func TestActionQueue_DequeueAll(t *testing.T) {
	q := newActionQueue()
	q.Enqueue("b_metric", "1")
	q.Enqueue("a_metric", "2")
	q.Enqueue("a_metric", "3") // latest wins

	actions := q.DequeueAll()
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	if actions[0].Metric != "a_metric" || actions[0].Value != "3" {
		t.Fatalf("unexpected actions[0]=%+v", actions[0])
	}
	if actions[1].Metric != "b_metric" || actions[1].Value != "1" {
		t.Fatalf("unexpected actions[1]=%+v", actions[1])
	}

	if again := q.DequeueAll(); len(again) != 0 {
		t.Fatalf("expected queue cleared, got %d", len(again))
	}
}
