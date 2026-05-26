package runtime

import (
	"encoding/json"
	"log/slog"
	"testing"

	"github.com/yttydcs/myflowhub-core/header"
	"github.com/yttydcs/myflowhub-proto/protocol/topicbus"
	"github.com/yttydcs/myflowhub-sdk/transport"

	"github.com/yttydcs/myflowhub-metricsnode/core/notify"
)

func TestNotifySettingsSet_NormalizesConfig(t *testing.T) {
	rt, err := New(t.TempDir(), slog.Default())
	if err != nil {
		t.Fatalf("runtime init failed: %v", err)
	}
	err = rt.NotifySettingsSet([]notify.TopicSetting{
		{Topic: " dev/codex/task ", Enabled: true},
		{Topic: "dev/codex/task", Enabled: false},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	got := rt.NotifySettingsGet()
	if len(got) != 1 || got[0].Topic != "dev/codex/task" || !got[0].Enabled {
		t.Fatalf("unexpected settings: %+v", got)
	}
}

func TestHandleTopicBusFrame_QueuesEnabledPublish(t *testing.T) {
	rt := &Runtime{
		notifyQ:      notify.NewQueue(8),
		notifyTopics: map[string]struct{}{"dev/codex/task": {}},
	}
	rt.notifyRunning.Store(true)
	hdr := (&header.HeaderTcp{}).WithMajor(header.MajorCmd).WithSubProto(topicbus.SubProtoTopicBus)
	payload, err := transport.EncodeMessage(topicbus.ActionPublish, topicbus.PublishReq{
		Topic:   "dev/codex/task",
		Name:    "done",
		TS:      123,
		Payload: json.RawMessage(`{"title":"Done","body":"Code finished"}`),
	})
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	rt.handleTopicBusFrame(hdr, payload)

	events := rt.DequeueNotifications()
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	if events[0].Title != "Done" || events[0].Body != "Code finished" {
		t.Fatalf("unexpected event: %+v", events[0])
	}
}

func TestHandleTopicBusFrame_IgnoresDisabledTopic(t *testing.T) {
	rt := &Runtime{
		notifyQ:      notify.NewQueue(8),
		notifyTopics: map[string]struct{}{"dev/codex/task": {}},
	}
	rt.notifyRunning.Store(true)
	hdr := (&header.HeaderTcp{}).WithMajor(header.MajorCmd).WithSubProto(topicbus.SubProtoTopicBus)
	payload, err := transport.EncodeMessage(topicbus.ActionPublish, topicbus.PublishReq{
		Topic: "other",
		Name:  "done",
	})
	if err != nil {
		t.Fatalf("encode failed: %v", err)
	}

	rt.handleTopicBusFrame(hdr, payload)

	if events := rt.DequeueNotifications(); len(events) != 0 {
		t.Fatalf("expected no events, got %d", len(events))
	}
}
