package notify

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const MaxTopicLength = 256

type TopicSetting struct {
	Topic   string `json:"topic"`
	Enabled bool   `json:"enabled"`
}

func DefaultSettingsJSON() string {
	return "[]"
}

func ParseSettingsJSON(text string) ([]TopicSetting, error) {
	raw := strings.TrimSpace(text)
	if raw == "" {
		return []TopicSetting{}, nil
	}
	var list []TopicSetting
	if err := json.Unmarshal([]byte(raw), &list); err != nil {
		return nil, err
	}
	return NormalizeSettings(list)
}

func SettingsJSON(settings []TopicSetting) (string, error) {
	normalized, err := NormalizeSettings(settings)
	if err != nil {
		return "", err
	}
	raw, err := json.Marshal(normalized)
	if err != nil {
		return "", err
	}
	return string(raw), nil
}

func NormalizeSettings(list []TopicSetting) ([]TopicSetting, error) {
	if len(list) == 0 {
		return []TopicSetting{}, nil
	}
	out := make([]TopicSetting, 0, len(list))
	seen := make(map[string]struct{}, len(list))
	for i, item := range list {
		topic := strings.TrimSpace(item.Topic)
		if topic == "" {
			return nil, fmt.Errorf("topic[%d] is required", i)
		}
		if len(topic) > MaxTopicLength {
			return nil, fmt.Errorf("topic[%d] exceeds %d bytes", i, MaxTopicLength)
		}
		if _, ok := seen[topic]; ok {
			continue
		}
		seen[topic] = struct{}{}
		out = append(out, TopicSetting{Topic: topic, Enabled: item.Enabled})
	}
	return out, nil
}

func EnabledTopics(settings []TopicSetting) []string {
	if len(settings) == 0 {
		return nil
	}
	out := make([]string, 0, len(settings))
	seen := make(map[string]struct{}, len(settings))
	for _, item := range settings {
		if !item.Enabled {
			continue
		}
		topic := strings.TrimSpace(item.Topic)
		if topic == "" {
			continue
		}
		if _, ok := seen[topic]; ok {
			continue
		}
		seen[topic] = struct{}{}
		out = append(out, topic)
	}
	return out
}

func EnabledTopicSet(settings []TopicSetting) map[string]struct{} {
	topics := EnabledTopics(settings)
	if len(topics) == 0 {
		return map[string]struct{}{}
	}
	out := make(map[string]struct{}, len(topics))
	for _, topic := range topics {
		out[topic] = struct{}{}
	}
	return out
}

func ValidateTopic(topic string) error {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return errors.New("topic is required")
	}
	if len(topic) > MaxTopicLength {
		return fmt.Errorf("topic exceeds %d bytes", MaxTopicLength)
	}
	return nil
}
