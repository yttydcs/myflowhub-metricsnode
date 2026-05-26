package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	core "github.com/yttydcs/myflowhub-core"
	"github.com/yttydcs/myflowhub-core/header"
	"github.com/yttydcs/myflowhub-proto/protocol/topicbus"
	"github.com/yttydcs/myflowhub-sdk/transport"

	"github.com/yttydcs/myflowhub-metricsnode/core/notify"
)

const defaultNotifyTimeout = 8 * time.Second

func (r *Runtime) NotifySettingsGet() []notify.TopicSetting {
	if r == nil {
		return nil
	}
	cfg := r.configSnapshot()
	out := make([]notify.TopicSetting, len(cfg.NotifySettings))
	copy(out, cfg.NotifySettings)
	return out
}

func (r *Runtime) NotifySettingsSet(settings []notify.TopicSetting) error {
	if r == nil {
		return errors.New("runtime not initialized")
	}
	raw, err := notify.SettingsJSON(settings)
	if err != nil {
		r.storeLastError(err)
		return err
	}
	return r.RuntimeConfigSet(KeyNotifyTopicsJSON, raw, 0)
}

func (r *Runtime) StartNotify() error {
	if r == nil {
		return errors.New("runtime not initialized")
	}
	auth := r.AuthState()
	if !auth.LoggedIn || auth.NodeID == 0 || auth.HubID == 0 {
		return errors.New("login required")
	}
	topics := notify.EnabledTopics(r.configSnapshot().NotifySettings)
	if len(topics) == 0 {
		return errors.New("notify topics are required")
	}
	if err := r.subscribeNotifyTopics(); err != nil {
		r.notifyRunning.Store(false)
		r.storeLastError(err)
		return err
	}
	r.notifyRunning.Store(true)
	if r.log != nil {
		r.log.Info("notify listener started", "topics", len(topics))
	}
	return nil
}

func (r *Runtime) StopNotify() {
	if r == nil {
		return
	}
	r.notifyRunning.Store(false)
	r.notifyMu.Lock()
	r.notifyTopics = map[string]struct{}{}
	r.notifyMu.Unlock()
	if r.log != nil {
		r.log.Info("notify listener stopped")
	}
}

func (r *Runtime) IsNotifyRunning() bool {
	if r == nil {
		return false
	}
	return r.notifyRunning.Load()
}

func (r *Runtime) DequeueNotifications() []notify.Event {
	if r == nil || r.notifyQ == nil {
		return nil
	}
	return r.notifyQ.DequeueAll()
}

func (r *Runtime) subscribeNotifyTopics() error {
	if r == nil {
		return errors.New("runtime not initialized")
	}
	auth := r.AuthState()
	if !auth.LoggedIn || auth.NodeID == 0 || auth.HubID == 0 {
		return errors.New("login required")
	}
	if !r.IsConnected() {
		return errors.New("not connected")
	}
	topics := notify.EnabledTopics(r.configSnapshot().NotifySettings)
	if len(topics) == 0 {
		r.notifyMu.Lock()
		r.notifyTopics = map[string]struct{}{}
		r.notifyMu.Unlock()
		return nil
	}
	payload, err := transport.EncodeMessage(topicbus.ActionSubscribeBatch, topicbus.SubscribeBatchReq{Topics: topics})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), defaultNotifyTimeout)
	defer cancel()
	resp, err := r.sendAndAwait(ctx, topicbus.SubProtoTopicBus, auth.NodeID, auth.HubID, payload, topicbus.ActionSubscribeBatchResp)
	if err != nil {
		return err
	}
	var out topicbus.Resp
	if err := json.Unmarshal(resp.Message.Data, &out); err != nil {
		return err
	}
	if out.Code != 1 {
		msg := strings.TrimSpace(out.Msg)
		if msg == "" {
			msg = fmt.Sprintf("topicbus subscribe_batch failed (code=%d)", out.Code)
		}
		return errors.New(msg)
	}
	r.notifyMu.Lock()
	r.notifyTopics = notify.EnabledTopicSet(r.configSnapshot().NotifySettings)
	r.notifyMu.Unlock()
	return nil
}

func (r *Runtime) tryHandleTopicBusFrame(hdr core.IHeader, payload []byte) bool {
	if r == nil || hdr == nil || len(payload) == 0 {
		return false
	}
	if hdr.SubProto() != topicbus.SubProtoTopicBus {
		return false
	}
	switch hdr.Major() {
	case header.MajorCmd, header.MajorMsg:
		// ok
	default:
		return false
	}
	r.handleTopicBusFrame(hdr, payload)
	return true
}

func (r *Runtime) handleTopicBusFrame(_ core.IHeader, payload []byte) {
	if r == nil || !r.IsNotifyRunning() {
		return
	}
	req, err := notify.ParsePublishEnvelope(payload)
	if err != nil {
		if r.log != nil {
			r.log.Warn("topicbus notify decode failed", "err", err.Error())
		}
		return
	}
	evt, ok := notify.EventFromPublish(req, r.isNotifyTopicEnabled, time.Now)
	if !ok {
		return
	}
	if r.notifyQ == nil {
		r.notifyQ = notify.NewQueue(notify.DefaultQueueCapacity)
	}
	r.notifyQ.Enqueue(evt)
	if r.log != nil {
		r.log.Info("notify event queued", "topic", evt.Topic, "name", evt.Name)
	}
}

func (r *Runtime) isNotifyTopicEnabled(topic string) bool {
	if r == nil {
		return false
	}
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return false
	}
	r.notifyMu.RLock()
	_, ok := r.notifyTopics[topic]
	r.notifyMu.RUnlock()
	if ok {
		return true
	}
	cfg := r.configSnapshot()
	for _, item := range cfg.NotifySettings {
		if item.Enabled && strings.TrimSpace(item.Topic) == topic {
			return true
		}
	}
	return false
}
