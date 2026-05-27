package main

import (
	"bytes"
	"context"
	_ "embed"
	"encoding/base64"
	"errors"
	"fmt"
	"html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf16"

	toast "git.sr.ht/~jackmordaunt/go-toast/v2"
	"git.sr.ht/~jackmordaunt/go-toast/v2/wintoast"
	"github.com/yttydcs/myflowhub-metricsnode/core/notify"
)

const (
	keyNotifyPresenter    = "notify.presenter"
	notifyPresenterScript = "script"
	notifyPresenterToast  = "toast"
	notifyAppID           = "MyFlowHub.MetricsNode"
	notifyToastGUID       = "{9F28742B-4D6D-4B0B-9A5C-EC6F79AD29B8}"
)

//go:embed frontend/src/assets/images/logo-universal.png
var notifyLogoPNG []byte

func (a *App) NotifyPresenterGet() (string, error) {
	a.mu.Lock()
	boot := a.boot
	a.mu.Unlock()
	if boot == nil {
		return notifyPresenterScript, nil
	}
	raw, _ := boot.Get(keyNotifyPresenter)
	mode, err := normalizeNotifyPresenter(raw)
	if err != nil {
		mode = notifyPresenterScript
		_ = boot.Set(keyNotifyPresenter, mode)
	}
	return mode, nil
}

func (a *App) NotifyPresenterSet(mode string) error {
	mode, err := normalizeNotifyPresenter(mode)
	if err != nil {
		return err
	}
	a.mu.Lock()
	boot := a.boot
	a.mu.Unlock()
	if boot == nil {
		return errors.New("bootstrap config not initialized")
	}
	return boot.Set(keyNotifyPresenter, mode)
}

func normalizeNotifyPresenter(mode string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(mode)) {
	case "", notifyPresenterScript:
		return notifyPresenterScript, nil
	case notifyPresenterToast:
		return notifyPresenterToast, nil
	default:
		return "", fmt.Errorf("unsupported notify presenter: %q", mode)
	}
}

func (a *App) ShowNotification(evt notify.Event) error {
	title, body, err := notificationText(evt)
	if err != nil {
		return err
	}
	mode, err := a.NotifyPresenterGet()
	if err != nil {
		return err
	}
	switch mode {
	case notifyPresenterToast:
		return a.showToastNotification(title, body)
	case notifyPresenterScript:
		return showScriptNotification(title, body)
	default:
		return fmt.Errorf("unsupported notify presenter: %q", mode)
	}
}

func notificationText(evt notify.Event) (string, string, error) {
	title := strings.TrimSpace(evt.Title)
	body := strings.TrimSpace(evt.Body)
	if title == "" {
		title = "MyFlowHub Notify"
	}
	if body == "" {
		body = strings.TrimSpace(evt.Topic)
	}
	if body == "" {
		body = strings.TrimSpace(evt.Name)
	}
	if body == "" {
		return "", "", errors.New("notification body is required")
	}
	return title, body, nil
}

func showScriptNotification(title string, body string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "powershell", "-NoProfile", "-ExecutionPolicy", "Bypass", "-WindowStyle", "Hidden", "-EncodedCommand", encodePowerShellCommand(notifyIconScript(title, body)))
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return fmt.Errorf("show notification timed out: %w", ctx.Err())
	}
	if err != nil {
		msg := strings.TrimSpace(string(out))
		if msg == "" {
			return err
		}
		return fmt.Errorf("%w: %s", err, msg)
	}
	return nil
}

func (a *App) showToastNotification(title string, body string) error {
	a.mu.Lock()
	iconPath := strings.TrimSpace(a.notifyIconPath)
	rt := a.rt
	a.mu.Unlock()
	if iconPath == "" {
		if rt != nil {
			path, err := ensureNotifyIcon(rt.WorkDir())
			if err != nil {
				return err
			}
			iconPath = path
			a.mu.Lock()
			a.notifyIconPath = path
			a.mu.Unlock()
		}
	}
	if iconPath == "" {
		return errors.New("notify icon path is required")
	}
	if err := toast.SetAppData(toast.AppData{
		AppID:               notifyAppID,
		GUID:                notifyToastGUID,
		IconPath:            iconPath,
		IconBackgroundColor: "#172436",
	}); err != nil {
		return err
	}
	xml := toastXML(title, body, iconPath)
	return wintoast.Push(notifyAppID, xml)
}

func ensureNotifyIcon(workDir string) (string, error) {
	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		return "", errors.New("workDir is required")
	}
	iconDir := filepath.Join(workDir, "assets")
	if err := os.MkdirAll(iconDir, 0o755); err != nil {
		return "", err
	}
	iconPath := filepath.Join(iconDir, "notify-logo.png")
	if sameFileContent(iconPath, notifyLogoPNG) {
		return iconPath, nil
	}
	if err := os.WriteFile(iconPath, notifyLogoPNG, 0o644); err != nil {
		return "", err
	}
	return iconPath, nil
}

func sameFileContent(path string, want []byte) bool {
	got, err := os.ReadFile(path)
	return err == nil && bytes.Equal(got, want)
}

func toastXML(title string, body string, iconPath string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<toast activationType="foreground" duration="short">
  <visual>
    <binding template="ToastGeneric">
      <image placement="appLogoOverride" src="%s" hint-crop="square" />
      <text hint-maxLines="1">%s</text>
      <text>%s</text>
    </binding>
  </visual>
  <audio src="ms-winsoundevent:Notification.Default" />
</toast>`, html.EscapeString(iconPath), html.EscapeString(title), html.EscapeString(body))
}

func notifyIconScript(title string, body string) string {
	titleB64 := base64.StdEncoding.EncodeToString([]byte(title))
	bodyB64 := base64.StdEncoding.EncodeToString([]byte(body))
	return fmt.Sprintf(`
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Windows.Forms
Add-Type -AssemblyName System.Drawing
$title = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$body = [System.Text.Encoding]::UTF8.GetString([System.Convert]::FromBase64String('%s'))
$n = New-Object System.Windows.Forms.NotifyIcon
$n.Icon = [System.Drawing.SystemIcons]::Information
$n.BalloonTipTitle = $title
$n.BalloonTipText = $body
$n.Visible = $true
$n.ShowBalloonTip(5000)
Start-Sleep -Milliseconds 5500
$n.Dispose()
`, titleB64, bodyB64)
}

func encodePowerShellCommand(script string) string {
	encoded := utf16.Encode([]rune(script))
	raw := make([]byte, len(encoded)*2)
	for i, v := range encoded {
		raw[i*2] = byte(v)
		raw[i*2+1] = byte(v >> 8)
	}
	return base64.StdEncoding.EncodeToString(raw)
}
