package notify

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/yttydcs/myflowhub-proto/protocol/topicbus"
	"github.com/yttydcs/myflowhub-sdk/transport"
)

func TestParsePublishEnvelope(t *testing.T) {
	body, err := transport.EncodeMessage(topicbus.ActionPublish, topicbus.PublishReq{
		Topic:   "dev/codex/task",
		Name:    "done",
		TS:      123,
		Payload: json.RawMessage(`{"title":"Build done","body":"All checks passed"}`),
	})
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}
	got, err := ParsePublishEnvelope(body)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Topic != "dev/codex/task" || got.Name != "done" || got.TS != 123 {
		t.Fatalf("unexpected publish: %+v", got)
	}
}

func TestEventFromPublish_ExtractTitleBody(t *testing.T) {
	evt, ok := EventFromPublish(topicbus.PublishReq{
		Topic:   "dev/codex/task",
		Name:    "done",
		Payload: json.RawMessage(`{"title":"Build done","body":"All checks passed"}`),
	}, func(topic string) bool { return topic == "dev/codex/task" }, func() time.Time {
		return time.UnixMilli(456)
	})
	if !ok {
		t.Fatalf("expected event")
	}
	if evt.Title != "Build done" || evt.Body != "All checks passed" || evt.TS != 456 {
		t.Fatalf("unexpected event: %+v", evt)
	}
}

func TestEventFromPublish_DisabledTopicIgnored(t *testing.T) {
	_, ok := EventFromPublish(topicbus.PublishReq{
		Topic: "other",
		Name:  "done",
	}, func(topic string) bool { return topic == "dev/codex/task" }, nil)
	if ok {
		t.Fatalf("expected disabled topic ignored")
	}
}

func TestEventFromPublish_PayloadPreviewFallback(t *testing.T) {
	evt, ok := EventFromPublish(topicbus.PublishReq{
		Topic:   "dev/codex/task",
		Name:    "done",
		TS:      1,
		Payload: json.RawMessage(`{"value":42}`),
	}, nil, nil)
	if !ok {
		t.Fatalf("expected event")
	}
	if evt.Title != "done" || evt.Body != `{"value":42}` {
		t.Fatalf("unexpected fallback: %+v", evt)
	}
}
