package runtime

import (
	"bufio"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/yttydcs/myflowhub-core/header"
	protoauth "github.com/yttydcs/myflowhub-proto/protocol/auth"
	"github.com/yttydcs/myflowhub-proto/protocol/topicbus"
	"github.com/yttydcs/myflowhub-sdk/transport"

	"github.com/yttydcs/myflowhub-metricsnode/core/notify"
)

type reconnectHubCounts struct {
	login     atomic.Int32
	subscribe atomic.Int32
}

func withFastReconnect(t *testing.T) {
	t.Helper()
	oldInitial := reconnectInitialDelay
	oldMax := reconnectMaxDelay
	reconnectInitialDelay = 10 * time.Millisecond
	reconnectMaxDelay = 20 * time.Millisecond
	t.Cleanup(func() {
		reconnectInitialDelay = oldInitial
		reconnectMaxDelay = oldMax
	})
}

func startReconnectHub(t *testing.T, closeAfterFirstSubscribe bool) (string, *reconnectHubCounts, func()) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	counts := &reconnectHubCounts{}
	stop := make(chan struct{})
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				select {
				case <-stop:
				default:
					t.Logf("accept stopped: %v", err)
				}
				return
			}
			go handleReconnectHubConn(t, conn, counts, closeAfterFirstSubscribe)
		}
	}()
	return ln.Addr().String(), counts, func() {
		close(stop)
		_ = ln.Close()
	}
}

func handleReconnectHubConn(t *testing.T, conn net.Conn, counts *reconnectHubCounts, closeAfterFirstSubscribe bool) {
	t.Helper()
	defer conn.Close()
	codec := header.HeaderTcpCodec{}
	reader := bufio.NewReader(conn)
	for {
		hdr, payload, err := codec.Decode(reader)
		if err != nil {
			return
		}
		msg, err := transport.DecodeMessage(payload)
		if err != nil {
			return
		}
		switch msg.Action {
		case protoauth.ActionLogin:
			n := counts.login.Add(1)
			resp, _ := transport.EncodeMessage(protoauth.ActionLoginResp, protoauth.RespData{
				Code:   1,
				Msg:    "ok",
				NodeID: 7,
				HubID:  1,
				Role:   "metrics",
			})
			writeReconnectHubResp(t, conn, codec, hdr, resp)
			if !closeAfterFirstSubscribe && n == 1 {
				time.Sleep(50 * time.Millisecond)
				return
			}
		case topicbus.ActionSubscribeBatch:
			n := counts.subscribe.Add(1)
			resp, _ := transport.EncodeMessage(topicbus.ActionSubscribeBatchResp, topicbus.Resp{
				Code: 1,
				Msg:  "ok",
			})
			writeReconnectHubResp(t, conn, codec, hdr, resp)
			if closeAfterFirstSubscribe && n == 1 {
				time.Sleep(50 * time.Millisecond)
				return
			}
		default:
			return
		}
	}
}

func writeReconnectHubResp(t *testing.T, conn net.Conn, codec header.HeaderTcpCodec, req coreHeader, payload []byte) {
	t.Helper()
	respHdr := (&header.HeaderTcp{}).
		WithMajor(header.MajorOKResp).
		WithSubProto(req.SubProto()).
		WithSourceID(req.TargetID()).
		WithTargetID(req.SourceID()).
		WithMsgID(req.GetMsgID()).
		WithPayloadLength(uint32(len(payload)))
	frame, err := codec.Encode(respHdr, payload)
	if err != nil {
		t.Fatalf("encode response: %v", err)
	}
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("write response: %v", err)
	}
}

type coreHeader interface {
	SubProto() uint8
	SourceID() uint32
	TargetID() uint32
	GetMsgID() uint32
}

func waitForReconnectCondition(t *testing.T, name string, fn func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("condition not met: %s", name)
}

func TestRuntimeAutoReconnect_ReloginsAfterSessionError(t *testing.T) {
	withFastReconnect(t)
	addr, counts, stop := startReconnectHub(t, false)
	defer stop()

	rt, err := New(t.TempDir(), slog.Default())
	if err != nil {
		t.Fatalf("runtime init failed: %v", err)
	}
	defer rt.Close()

	if _, err := rt.EnsureKeys(); err != nil {
		t.Fatalf("ensure keys: %v", err)
	}
	if err := rt.Connect(addr); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := rt.Login("device-a", 7); err != nil {
		t.Fatalf("login: %v", err)
	}

	waitForReconnectCondition(t, "second login after reconnect", func() bool {
		return counts.login.Load() >= 2 && rt.IsConnected() && rt.AuthState().LoggedIn
	})
}

func TestRuntimeAutoReconnect_ResubscribesNotifyAfterReconnect(t *testing.T) {
	withFastReconnect(t)
	addr, counts, stop := startReconnectHub(t, true)
	defer stop()

	rt, err := New(t.TempDir(), slog.Default())
	if err != nil {
		t.Fatalf("runtime init failed: %v", err)
	}
	defer rt.Close()

	if _, err := rt.EnsureKeys(); err != nil {
		t.Fatalf("ensure keys: %v", err)
	}
	if err := rt.NotifySettingsSet([]notify.TopicSetting{{Topic: "dev/test", Enabled: true}}); err != nil {
		t.Fatalf("notify settings: %v", err)
	}
	if err := rt.Connect(addr); err != nil {
		t.Fatalf("connect: %v", err)
	}
	if _, err := rt.Login("device-a", 7); err != nil {
		t.Fatalf("login: %v", err)
	}
	if err := rt.StartNotify(); err != nil {
		t.Fatalf("start notify: %v", err)
	}

	waitForReconnectCondition(t, "second subscribe after reconnect", func() bool {
		return counts.login.Load() >= 2 && counts.subscribe.Load() >= 2 && rt.IsConnected() && rt.IsNotifyRunning()
	})
}

func TestRuntimeLoadAuthSnapshot_RequiresFreshLogin(t *testing.T) {
	dir := t.TempDir()
	raw, err := json.Marshal(AuthSnapshot{
		DeviceID: "device-a",
		NodeID:   7,
		HubID:    1,
		Role:     "metrics",
		LoggedIn: true,
	})
	if err != nil {
		t.Fatalf("marshal auth snapshot: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "auth_snapshot.json"), raw, 0o600); err != nil {
		t.Fatalf("write auth snapshot: %v", err)
	}

	rt, err := New(dir, slog.Default())
	if err != nil {
		t.Fatalf("runtime init failed: %v", err)
	}

	st := rt.AuthState()
	if st.LoggedIn {
		t.Fatalf("loaded auth snapshot must require fresh login: %+v", st)
	}
	if st.DeviceID != "device-a" || st.NodeID != 7 || st.HubID != 1 {
		t.Fatalf("identity fields should be preserved: %+v", st)
	}
}
