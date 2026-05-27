package main

import (
	"encoding/base64"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/yttydcs/myflowhub-metricsnode/core/notify"
)

func TestEncodePowerShellCommand_UTF16LE(t *testing.T) {
	got := encodePowerShellCommand("Write-Output 'ok'")
	raw, err := base64.StdEncoding.DecodeString(got)
	if err != nil {
		t.Fatalf("decode encoded command: %v", err)
	}
	if len(raw)%2 != 0 {
		t.Fatalf("encoded command has odd byte length: %d", len(raw))
	}
	words := make([]uint16, len(raw)/2)
	for i := range words {
		words[i] = uint16(raw[i*2]) | uint16(raw[i*2+1])<<8
	}
	if decoded := string(utf16.Decode(words)); decoded != "Write-Output 'ok'" {
		t.Fatalf("decoded command mismatch: %q", decoded)
	}
}

func TestNormalizeNotifyPresenter(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want string
	}{
		{"", notifyPresenterScript},
		{"script", notifyPresenterScript},
		{"SCRIPT", notifyPresenterScript},
		{"toast", notifyPresenterToast},
		{" Toast ", notifyPresenterToast},
	} {
		got, err := normalizeNotifyPresenter(tc.in)
		if err != nil {
			t.Fatalf("normalizeNotifyPresenter(%q): %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("normalizeNotifyPresenter(%q)=%q want %q", tc.in, got, tc.want)
		}
	}
	if _, err := normalizeNotifyPresenter("native"); err == nil {
		t.Fatal("expected invalid presenter error")
	}
}

func TestNotificationTextFallback(t *testing.T) {
	title, body, err := notificationText(notify.Event{Topic: "dev.codex.msg", Name: "done"})
	if err != nil {
		t.Fatalf("notificationText: %v", err)
	}
	if title != "MyFlowHub Notify" || body != "dev.codex.msg" {
		t.Fatalf("unexpected title/body: %q %q", title, body)
	}
}

func TestToastXML_EscapesTextAndIcon(t *testing.T) {
	xml := toastXML(`A "quote" & <tag>`, `Body & <x>`, `C:\Temp\icon & logo.png`)
	for _, bad := range []string{`A "quote" & <tag>`, `Body & <x>`, `icon & logo.png`} {
		if strings.Contains(xml, bad) {
			t.Fatalf("toast XML contains unescaped text %q: %s", bad, xml)
		}
	}
	for _, want := range []string{"ToastGeneric", "appLogoOverride", "A &#34;quote&#34; &amp; &lt;tag&gt;", "Body &amp; &lt;x&gt;"} {
		if !strings.Contains(xml, want) {
			t.Fatalf("toast XML missing %q: %s", want, xml)
		}
	}
}

func TestNotifyIconScript_EmbedsBase64Text(t *testing.T) {
	script := notifyIconScript("Codex 完成任务", "NotifyNode 已经构建完成")
	if strings.Contains(script, "Codex 完成任务") || strings.Contains(script, "NotifyNode 已经构建完成") {
		t.Fatalf("script should embed notification text as base64, got %q", script)
	}
	for _, want := range []string{
		"[System.Convert]::FromBase64String",
		"BalloonTipTitle",
		"BalloonTipText",
		"ShowBalloonTip(5000)",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("script missing %q: %q", want, script)
		}
	}
}
