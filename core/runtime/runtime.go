package runtime

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	core "github.com/yttydcs/myflowhub-core"
	"github.com/yttydcs/myflowhub-core/header"
	protoauth "github.com/yttydcs/myflowhub-proto/protocol/auth"
	protovar "github.com/yttydcs/myflowhub-proto/protocol/varstore"
	sdkawait "github.com/yttydcs/myflowhub-sdk/await"
	"github.com/yttydcs/myflowhub-sdk/session"
	"github.com/yttydcs/myflowhub-sdk/transport"

	rtauth "github.com/yttydcs/myflowhub-metricsnode/core/auth"
	"github.com/yttydcs/myflowhub-metricsnode/core/configstore"
	"github.com/yttydcs/myflowhub-metricsnode/core/metrics"
	rtvar "github.com/yttydcs/myflowhub-metricsnode/core/varstore"
)

const defaultAuthTimeout = 8 * time.Second

type AuthSnapshot struct {
	DeviceID string `json:"device_id,omitempty"`
	NodeID   uint32 `json:"node_id,omitempty"`
	HubID    uint32 `json:"hub_id,omitempty"`
	Role     string `json:"role,omitempty"`

	LoggedIn     bool   `json:"logged_in"`
	LastAction   string `json:"last_action,omitempty"`
	LastMessage  string `json:"last_message,omitempty"`
	LastUnixTime int64  `json:"last_unix_time,omitempty"`
}

type publishedVar struct {
	Value      string
	Visibility string
}

type Runtime struct {
	log *slog.Logger

	workDir string

	clientMu sync.Mutex
	client   *sdkawait.Client
	addr     string

	connected atomic.Bool

	authMu sync.Mutex
	auth   AuthSnapshot

	lastErr atomic.Value // string

	keys *rtauth.KeyStore

	cfgStore *configstore.Store
	cfgMu    sync.RWMutex
	cfg      runtimeConfig

	reportMu      sync.Mutex
	reportCancel  context.CancelFunc
	reportEnabled atomic.Bool
	lastMetrics   map[string]string
	lastPublished map[string]publishedVar
}

func New(workDir string, log *slog.Logger) (*Runtime, error) {
	workDir = strings.TrimSpace(workDir)
	if workDir == "" {
		return nil, errors.New("workDir is required")
	}
	abs := workDir
	if !filepath.IsAbs(abs) {
		if wd, err := os.Getwd(); err == nil && strings.TrimSpace(wd) != "" {
			abs = filepath.Join(wd, workDir)
		}
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return nil, err
	}
	if log == nil {
		log = slog.Default()
	}
	rt := &Runtime{
		log:     log,
		workDir: abs,
	}
	rt.keys = rtauth.NewKeyStore(filepath.Join(abs, "node_keys.json"))
	_ = rt.loadAuthSnapshot()
	if err := rt.initRuntimeConfig(); err != nil {
		return nil, err
	}
	return rt, nil
}

func (r *Runtime) WorkDir() string {
	if r == nil {
		return ""
	}
	return r.workDir
}

func (r *Runtime) LastError() string {
	if r == nil {
		return ""
	}
	if v := r.lastErr.Load(); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func (r *Runtime) storeLastError(err error) {
	if r == nil {
		return
	}
	if err == nil {
		r.lastErr.Store("")
		return
	}
	r.lastErr.Store(err.Error())
}

func (r *Runtime) ensureClient() *sdkawait.Client {
	r.clientMu.Lock()
	defer r.clientMu.Unlock()
	if r.client != nil {
		return r.client
	}
	c := sdkawait.NewClient(context.Background(), r.onUnmatchedFrame, r.onClientError)
	r.client = c
	return c
}

func (r *Runtime) onUnmatchedFrame(hdr core.IHeader, payload []byte) {
	if r == nil || hdr == nil || len(payload) == 0 {
		return
	}
	if r.tryHandleManagementFrame(hdr, payload) {
		return
	}
	if r.log == nil || !r.log.Enabled(context.Background(), slog.LevelDebug) {
		return
	}
	preview := payload
	truncated := false
	if len(preview) > 256 {
		preview = preview[:256]
		truncated = true
	}
	r.log.Debug("rx unmatched",
		"major", hdr.Major(),
		"sub", hdr.SubProto(),
		"src", hdr.SourceID(),
		"tgt", hdr.TargetID(),
		"len", len(payload),
		"preview", string(preview),
		"truncated", truncated,
	)
}

func (r *Runtime) onClientError(err error) {
	if r == nil || err == nil {
		return
	}
	r.connected.Store(false)
	r.storeLastError(err)
	if r.log != nil {
		r.log.Warn("client session error", "err", err.Error())
	}
}

func (r *Runtime) Connect(addr string) error {
	if r == nil {
		return errors.New("runtime not initialized")
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return errors.New("addr is required")
	}

	r.clientMu.Lock()
	prevAddr := r.addr
	r.clientMu.Unlock()
	if prevAddr != "" && prevAddr != addr {
		r.Close()
	}

	c := r.ensureClient()
	if err := c.Connect(addr); err != nil {
		if errors.Is(err, session.ErrAlreadyConnected) {
			r.connected.Store(true)
			r.clientMu.Lock()
			r.addr = addr
			r.clientMu.Unlock()
			return nil
		}
		r.storeLastError(err)
		if r.log != nil {
			r.log.Warn("client connect failed", "addr", addr, "err", err.Error())
		}
		return err
	}

	r.connected.Store(true)
	r.clientMu.Lock()
	r.addr = addr
	r.clientMu.Unlock()
	if r.log != nil {
		r.log.Info("client connected", "addr", addr)
	}
	return nil
}

func (r *Runtime) Close() {
	if r == nil {
		return
	}
	r.StopReporting()
	r.clientMu.Lock()
	c := r.client
	r.client = nil
	r.addr = ""
	r.clientMu.Unlock()

	if c != nil {
		c.Close()
	}
	r.connected.Store(false)
	if r.log != nil {
		r.log.Info("client closed")
	}
}

func (r *Runtime) IsConnected() bool {
	if r == nil {
		return false
	}
	return r.connected.Load()
}

func (r *Runtime) LastAddr() string {
	if r == nil {
		return ""
	}
	r.clientMu.Lock()
	addr := r.addr
	r.clientMu.Unlock()
	return addr
}

func (r *Runtime) EnsureKeys() (string, error) {
	if r == nil || r.keys == nil {
		return "", errors.New("runtime not initialized")
	}
	pub, err := r.keys.Ensure()
	if err != nil {
		r.storeLastError(err)
	}
	return pub, err
}

func (r *Runtime) AuthState() AuthSnapshot {
	if r == nil {
		return AuthSnapshot{}
	}
	r.authMu.Lock()
	st := r.auth
	r.authMu.Unlock()
	return st
}

func (r *Runtime) ClearAuth() error {
	if r == nil {
		return errors.New("runtime not initialized")
	}
	r.authMu.Lock()
	r.auth = AuthSnapshot{}
	r.authMu.Unlock()
	if err := r.saveAuthSnapshot(AuthSnapshot{}); err != nil {
		r.storeLastError(err)
		return err
	}
	if r.log != nil {
		r.log.Info("auth state cleared")
	}
	return nil
}

func (r *Runtime) Register(deviceID string) (protoauth.RespData, error) {
	if r == nil {
		return protoauth.RespData{}, errors.New("runtime not initialized")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return protoauth.RespData{}, errors.New("device_id is required")
	}
	if !r.IsConnected() {
		return protoauth.RespData{}, errors.New("not connected")
	}

	pub, err := r.EnsureKeys()
	if err != nil {
		return protoauth.RespData{}, err
	}

	payload, err := transport.EncodeMessage(protoauth.ActionRegister, protoauth.RegisterData{
		DeviceID: deviceID,
		PubKey:   pub,
		NodePub:  pub,
	})
	if err != nil {
		return protoauth.RespData{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultAuthTimeout)
	defer cancel()
	resp, err := r.sendAndAwait(ctx, protoauth.SubProtoAuth, 0, 0, payload, protoauth.ActionRegisterResp)
	if err != nil {
		r.storeLastError(err)
		r.setAuthResult(false, protoauth.ActionRegisterResp, err.Error())
		return protoauth.RespData{}, err
	}

	var data protoauth.RespData
	if err := json.Unmarshal(resp.Message.Data, &data); err != nil {
		r.storeLastError(err)
		r.setAuthResult(false, protoauth.ActionRegisterResp, err.Error())
		return protoauth.RespData{}, err
	}
	if data.Code != 1 {
		msg := strings.TrimSpace(data.Msg)
		if msg == "" {
			msg = fmt.Sprintf("auth register failed (code=%d)", data.Code)
		}
		err := errors.New(msg)
		r.storeLastError(err)
		r.setAuthResult(false, protoauth.ActionRegisterResp, msg)
		return protoauth.RespData{}, err
	}

	r.setAuthSnapshot(protoauth.ActionRegisterResp, deviceID, data)
	if err := r.saveAuthSnapshot(r.AuthState()); err != nil {
		r.storeLastError(err)
	}

	return data, nil
}

func (r *Runtime) Login(deviceID string, nodeID uint32) (protoauth.RespData, error) {
	if r == nil {
		return protoauth.RespData{}, errors.New("runtime not initialized")
	}
	deviceID = strings.TrimSpace(deviceID)
	if deviceID == "" {
		return protoauth.RespData{}, errors.New("device_id is required")
	}
	if nodeID == 0 {
		return protoauth.RespData{}, errors.New("node_id is required")
	}
	if !r.IsConnected() {
		return protoauth.RespData{}, errors.New("not connected")
	}

	ts := time.Now().Unix()
	nonce := rtauth.GenerateNonce(12)
	sig, err := r.keys.SignLogin(deviceID, nodeID, ts, nonce)
	if err != nil {
		r.storeLastError(err)
		return protoauth.RespData{}, err
	}

	payload, err := transport.EncodeMessage(protoauth.ActionLogin, protoauth.LoginData{
		DeviceID: deviceID,
		NodeID:   nodeID,
		TS:       ts,
		Nonce:    nonce,
		Sig:      sig,
		Alg:      "ES256",
	})
	if err != nil {
		return protoauth.RespData{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), defaultAuthTimeout)
	defer cancel()
	resp, err := r.sendAndAwait(ctx, protoauth.SubProtoAuth, 0, 0, payload, protoauth.ActionLoginResp)
	if err != nil {
		r.storeLastError(err)
		r.setAuthResult(false, protoauth.ActionLoginResp, err.Error())
		return protoauth.RespData{}, err
	}

	var data protoauth.RespData
	if err := json.Unmarshal(resp.Message.Data, &data); err != nil {
		r.storeLastError(err)
		r.setAuthResult(false, protoauth.ActionLoginResp, err.Error())
		return protoauth.RespData{}, err
	}
	if data.Code != 1 {
		msg := strings.TrimSpace(data.Msg)
		if msg == "" {
			msg = fmt.Sprintf("auth login failed (code=%d)", data.Code)
		}
		err := errors.New(msg)
		r.storeLastError(err)
		r.setAuthResult(false, protoauth.ActionLoginResp, msg)
		return protoauth.RespData{}, err
	}

	r.setAuthSnapshot(protoauth.ActionLoginResp, deviceID, data)
	if err := r.saveAuthSnapshot(r.AuthState()); err != nil {
		r.storeLastError(err)
	}

	return data, nil
}

func (r *Runtime) IsReporting() bool {
	if r == nil {
		return false
	}
	return r.reportEnabled.Load()
}

func (r *Runtime) MetricsSnapshot() map[string]string {
	if r == nil {
		return nil
	}
	r.reportMu.Lock()
	if len(r.lastMetrics) == 0 {
		r.reportMu.Unlock()
		return map[string]string{}
	}
	out := make(map[string]string, len(r.lastMetrics))
	for k, v := range r.lastMetrics {
		out[k] = v
	}
	r.reportMu.Unlock()
	return out
}

func (r *Runtime) StartReporting() error {
	if r == nil {
		return errors.New("runtime not initialized")
	}
	auth := r.AuthState()
	if !auth.LoggedIn || auth.NodeID == 0 || auth.HubID == 0 {
		return errors.New("login required")
	}

	r.reportMu.Lock()
	defer r.reportMu.Unlock()
	if r.reportCancel != nil {
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	r.reportCancel = cancel
	r.reportEnabled.Store(true)
	if r.lastMetrics == nil {
		r.lastMetrics = make(map[string]string)
	}
	if r.lastPublished == nil {
		r.lastPublished = make(map[string]publishedVar)
	}

	metrics.StartPlatformCollectors(ctx, r.log, r.handleMetricUpdate)
	if r.log != nil {
		r.log.Info("metrics reporting started")
	}
	return nil
}

func (r *Runtime) StopReporting() {
	if r == nil {
		return
	}
	r.reportMu.Lock()
	cancel := r.reportCancel
	r.reportCancel = nil
	r.reportMu.Unlock()
	if cancel != nil {
		cancel()
	}
	r.reportEnabled.Store(false)
	if r.log != nil {
		r.log.Info("metrics reporting stopped")
	}
}

// UpdateMetric allows external platforms (e.g. Android) to feed metric values into the runtime.
func (r *Runtime) UpdateMetric(metric string, value string) {
	if r == nil {
		return
	}
	r.handleMetricUpdate(strings.TrimSpace(metric), strings.TrimSpace(value))
}

func (r *Runtime) handleMetricUpdate(metric string, value string) {
	if r == nil {
		return
	}
	metric = strings.TrimSpace(metric)
	rawValue := strings.TrimSpace(value)
	if metric == "" || rawValue == "" {
		return
	}
	if !r.IsReporting() {
		return
	}
	auth := r.AuthState()
	if !auth.LoggedIn || auth.NodeID == 0 || auth.HubID == 0 {
		return
	}

	r.reportMu.Lock()
	if r.lastMetrics == nil {
		r.lastMetrics = make(map[string]string)
	}
	r.lastMetrics[metric] = rawValue
	r.reportMu.Unlock()

	cfg := r.configSnapshot()
	if len(cfg.Bindings) == 0 {
		return
	}

	visibility := strings.TrimSpace(cfg.VisibilityDefault)
	if visibility == "" {
		visibility = protovar.VisibilityPublic
	}
	publishValue := transformMetricValue(metric, rawValue, cfg)
	if publishValue == "" {
		return
	}
	for _, b := range cfg.Bindings {
		if b.Metric != metric {
			continue
		}
		r.publishVar(auth.NodeID, auth.HubID, b.VarName, publishValue, visibility)
	}
}

func (r *Runtime) publishVar(sourceID, targetID uint32, varName, value, visibility string) {
	if r == nil {
		return
	}
	varName = strings.TrimSpace(varName)
	value = strings.TrimSpace(value)
	visibility = strings.ToLower(strings.TrimSpace(visibility))
	if varName == "" || value == "" {
		return
	}
	if visibility == "" {
		visibility = protovar.VisibilityPublic
	}
	if !rtvar.ValidVarName(varName) {
		if r.log != nil {
			r.log.Warn("invalid var name; drop update", "var", varName)
		}
		return
	}

	r.reportMu.Lock()
	if r.lastPublished == nil {
		r.lastPublished = make(map[string]publishedVar)
	}
	prev, ok := r.lastPublished[varName]
	if ok && prev.Value == value && prev.Visibility == visibility {
		r.reportMu.Unlock()
		return
	}
	r.lastPublished[varName] = publishedVar{Value: value, Visibility: visibility}
	r.reportMu.Unlock()

	vc := rtvar.New(r.ensureClient(), r.log)
	req := protovar.SetReq{
		Name:       varName,
		Value:      value,
		Visibility: visibility,
		Type:       "string",
		Owner:      sourceID,
	}
	if err := vc.Set(sourceID, targetID, req); err != nil {
		r.storeLastError(err)
		if r.log != nil {
			r.log.Warn("varstore set failed", "var", varName, "err", err.Error())
		}
	}
}

func (r *Runtime) sendAndAwait(ctx context.Context, sub uint8, src, tgt uint32, payload []byte, expectAction string) (sdkawait.Response, error) {
	c := r.ensureClient()
	hdr := (&header.HeaderTcp{}).
		WithMajor(header.MajorCmd).
		WithSubProto(sub).
		WithSourceID(src).
		WithTargetID(tgt).
		WithTimestamp(uint32(time.Now().Unix()))
	resp, err := c.SendAndAwait(ctx, hdr, payload, expectAction)
	if err != nil {
		return sdkawait.Response{}, fmt.Errorf("%s: %w", expectAction, toUIError(err))
	}
	return resp, nil
}

func (r *Runtime) setAuthSnapshot(action, deviceID string, data protoauth.RespData) {
	r.authMu.Lock()
	r.auth = AuthSnapshot{
		DeviceID:     deviceID,
		NodeID:       data.NodeID,
		HubID:        data.HubID,
		Role:         strings.TrimSpace(data.Role),
		LoggedIn:     true,
		LastAction:   strings.TrimSpace(action),
		LastMessage:  strings.TrimSpace(data.Msg),
		LastUnixTime: time.Now().Unix(),
	}
	r.authMu.Unlock()
	if r.log != nil {
		r.log.Info("auth ok",
			"action", strings.TrimSpace(action),
			"device", deviceID,
			"node", data.NodeID,
			"hub", data.HubID,
			"role", strings.TrimSpace(data.Role),
		)
	}
}

func (r *Runtime) setAuthResult(ok bool, action, msg string) {
	r.authMu.Lock()
	st := r.auth
	st.LoggedIn = ok
	st.LastAction = strings.TrimSpace(action)
	st.LastMessage = strings.TrimSpace(msg)
	st.LastUnixTime = time.Now().Unix()
	r.auth = st
	r.authMu.Unlock()
}

func (r *Runtime) authSnapshotPath() string {
	if r == nil {
		return ""
	}
	return filepath.Join(r.workDir, "auth_snapshot.json")
}

func (r *Runtime) loadAuthSnapshot() error {
	path := r.authSnapshotPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var st AuthSnapshot
	if err := json.Unmarshal(data, &st); err != nil {
		return err
	}
	r.authMu.Lock()
	r.auth = st
	r.authMu.Unlock()
	return nil
}

func (r *Runtime) saveAuthSnapshot(st AuthSnapshot) error {
	path := r.authSnapshotPath()
	if path == "" {
		return errors.New("auth snapshot path invalid")
	}
	raw, _ := json.MarshalIndent(st, "", "  ")
	return writeFileAtomic(path, raw, 0o600)
}

func toUIError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.New("request timed out")
	}
	if errors.Is(err, context.Canceled) {
		return errors.New("request canceled")
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "session not initialized"):
		return errors.New("not connected")
	case strings.Contains(msg, "connection") && strings.Contains(msg, "closed"):
		return errors.New("connection closed")
	default:
		return err
	}
}

func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	path = filepath.Clean(path)
	if path == "" {
		return errors.New("path is required")
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, perm); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err == nil {
		return nil
	}
	_ = os.Remove(path)
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}
