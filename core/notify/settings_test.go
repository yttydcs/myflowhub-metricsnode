package notify

import "testing"

func TestNormalizeSettings_TrimDeduplicate(t *testing.T) {
	got, err := NormalizeSettings([]TopicSetting{
		{Topic: " dev/codex/task ", Enabled: true},
		{Topic: "dev/codex/task", Enabled: false},
		{Topic: "home/alert", Enabled: true},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 settings, got %d", len(got))
	}
	if got[0].Topic != "dev/codex/task" || !got[0].Enabled {
		t.Fatalf("unexpected first setting: %+v", got[0])
	}
}

func TestNormalizeSettings_BlankRejected(t *testing.T) {
	if _, err := NormalizeSettings([]TopicSetting{{Topic: " "}}); err == nil {
		t.Fatalf("expected error")
	}
}

func TestEnabledTopics(t *testing.T) {
	got := EnabledTopics([]TopicSetting{
		{Topic: "a", Enabled: true},
		{Topic: "b", Enabled: false},
		{Topic: "a", Enabled: true},
	})
	if len(got) != 1 || got[0] != "a" {
		t.Fatalf("unexpected enabled topics: %+v", got)
	}
}
