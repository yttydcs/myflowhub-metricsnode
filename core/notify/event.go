package notify

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/yttydcs/myflowhub-proto/protocol/topicbus"
	"github.com/yttydcs/myflowhub-sdk/transport"
)

const (
	MaxTitleLength       = 96
	MaxBodyLength        = 220
	MaxPayloadTextLength = 180
	MaxPayloadBytes      = 32 * 1024
)

type Event struct {
	ID      string          `json:"id"`
	Topic   string          `json:"topic"`
	Name    string          `json:"name"`
	Title   string          `json:"title"`
	Body    string          `json:"body"`
	TS      int64           `json:"ts"`
	Payload json.RawMessage `json:"payload,omitempty"`
}

func ParsePublishEnvelope(payload []byte) (topicbus.PublishReq, error) {
	if len(payload) == 0 {
		return topicbus.PublishReq{}, fmt.Errorf("payload is required")
	}
	if len(payload) > MaxPayloadBytes {
		return topicbus.PublishReq{}, fmt.Errorf("payload exceeds %d bytes", MaxPayloadBytes)
	}
	msg, err := transport.DecodeMessage(payload)
	if err != nil {
		return topicbus.PublishReq{}, err
	}
	if strings.TrimSpace(msg.Action) != topicbus.ActionPublish {
		return topicbus.PublishReq{}, fmt.Errorf("unexpected topicbus action: %s", msg.Action)
	}
	var req topicbus.PublishReq
	if err := json.Unmarshal(msg.Data, &req); err != nil {
		return topicbus.PublishReq{}, err
	}
	req.Topic = strings.TrimSpace(req.Topic)
	req.Name = strings.TrimSpace(req.Name)
	if req.Topic == "" {
		return topicbus.PublishReq{}, fmt.Errorf("topic is required")
	}
	if req.Name == "" {
		return topicbus.PublishReq{}, fmt.Errorf("name is required")
	}
	return req, nil
}

func EventFromPublish(req topicbus.PublishReq, topicEnabled func(string) bool, now func() time.Time) (Event, bool) {
	req.Topic = strings.TrimSpace(req.Topic)
	req.Name = strings.TrimSpace(req.Name)
	if req.Topic == "" || req.Name == "" {
		return Event{}, false
	}
	if topicEnabled != nil && !topicEnabled(req.Topic) {
		return Event{}, false
	}
	ts := req.TS
	if ts == 0 {
		if now == nil {
			now = time.Now
		}
		ts = now().UnixMilli()
	}
	title, body := extractTitleBody(req)
	id := fmt.Sprintf("%d:%s:%s", ts, req.Topic, req.Name)
	payloadCopy := append([]byte(nil), req.Payload...)
	return Event{
		ID:      limitText(id, MaxTitleLength+MaxBodyLength),
		Topic:   req.Topic,
		Name:    req.Name,
		Title:   title,
		Body:    body,
		TS:      ts,
		Payload: payloadCopy,
	}, true
}

func extractTitleBody(req topicbus.PublishReq) (string, string) {
	title := ""
	body := ""

	if len(req.Payload) > 0 && string(req.Payload) != "null" {
		var obj map[string]any
		if err := json.Unmarshal(req.Payload, &obj); err == nil {
			title = firstString(obj, "title", "subject", "summary")
			body = firstString(obj, "body", "message", "text", "content", "summary")
		}
	}
	if strings.TrimSpace(title) == "" {
		title = req.Name
	}
	if strings.TrimSpace(body) == "" {
		body = payloadPreview(req.Payload)
	}
	if strings.TrimSpace(body) == "" {
		body = req.Topic
	}
	return limitText(title, MaxTitleLength), limitText(body, MaxBodyLength)
}

func firstString(obj map[string]any, keys ...string) string {
	for _, key := range keys {
		v, ok := obj[key]
		if !ok {
			continue
		}
		switch vv := v.(type) {
		case string:
			if s := strings.TrimSpace(vv); s != "" {
				return s
			}
		case float64, bool:
			return strings.TrimSpace(fmt.Sprint(vv))
		}
	}
	return ""
}

func payloadPreview(raw json.RawMessage) string {
	if len(raw) == 0 {
		return ""
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return limitText(string(raw), MaxPayloadTextLength)
	}
	compact, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	return limitText(string(compact), MaxPayloadTextLength)
}

func limitText(text string, max int) string {
	text = strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	if max <= 0 || len(text) <= max {
		return text
	}
	if max <= 3 {
		return text[:max]
	}
	return text[:max-3] + "..."
}
