package runtime

// 本文件承载 MetricsNode 应用层中与 `management` 相关的逻辑。

import (
	"encoding/json"
	"strings"

	core "github.com/yttydcs/myflowhub-core"
	"github.com/yttydcs/myflowhub-core/header"
	protomgmt "github.com/yttydcs/myflowhub-proto/protocol/management"
	"github.com/yttydcs/myflowhub-sdk/transport"
)

// tryHandleManagementFrame 只拦截 management 的命令帧，把其他协议继续留给常规分发链。
func (r *Runtime) tryHandleManagementFrame(hdr core.IHeader, payload []byte) bool {
	if r == nil || hdr == nil || len(payload) == 0 {
		return false
	}
	if hdr.SubProto() != protomgmt.SubProtoManagement {
		return false
	}
	if hdr.Major() != header.MajorCmd {
		return false
	}
	r.handleManagementCmd(hdr, payload)
	return true
}

// handleManagementCmd 处理本节点可本地回答的配置查询/写入请求，避免额外转发。
func (r *Runtime) handleManagementCmd(hdr core.IHeader, payload []byte) {
	if r == nil || hdr == nil || len(payload) == 0 {
		return
	}

	msg, err := transport.DecodeMessage(payload)
	if err != nil {
		if r.log != nil {
			r.log.Warn("management decode failed", "err", err.Error())
		}
		return
	}

	switch msg.Action {
	case protomgmt.ActionConfigList:
		keys := r.RuntimeConfigKeys()
		if keys == nil {
			r.sendManagementResp(hdr, protomgmt.ActionConfigListResp, protomgmt.ConfigListResp{Code: 500, Msg: "config unavailable"})
			return
		}
		r.sendManagementResp(hdr, protomgmt.ActionConfigListResp, protomgmt.ConfigListResp{Code: 1, Msg: "ok", Keys: keys})
	case protomgmt.ActionConfigGet:
		var req protomgmt.ConfigGetReq
		if err := json.Unmarshal(msg.Data, &req); err != nil || strings.TrimSpace(req.Key) == "" {
			r.sendManagementResp(hdr, protomgmt.ActionConfigGetResp, protomgmt.ConfigResp{Code: 400, Msg: "invalid key"})
			return
		}
		key := strings.TrimSpace(req.Key)
		val, ok := r.RuntimeConfigGet(key)
		if !ok {
			r.sendManagementResp(hdr, protomgmt.ActionConfigGetResp, protomgmt.ConfigResp{Code: 404, Msg: "not found", Key: key})
			return
		}
		r.sendManagementResp(hdr, protomgmt.ActionConfigGetResp, protomgmt.ConfigResp{Code: 1, Msg: "ok", Key: key, Value: val})
	case protomgmt.ActionConfigSet:
		var req protomgmt.ConfigSetReq
		if err := json.Unmarshal(msg.Data, &req); err != nil || strings.TrimSpace(req.Key) == "" {
			r.sendManagementResp(hdr, protomgmt.ActionConfigSetResp, protomgmt.ConfigResp{Code: 400, Msg: "invalid key"})
			return
		}
		key := strings.TrimSpace(req.Key)
		if err := r.RuntimeConfigSet(key, req.Value, hdr.SourceID()); err != nil {
			r.sendManagementResp(hdr, protomgmt.ActionConfigSetResp, protomgmt.ConfigResp{Code: 400, Msg: err.Error(), Key: key, Value: req.Value})
			return
		}
		r.sendManagementResp(hdr, protomgmt.ActionConfigSetResp, protomgmt.ConfigResp{Code: 1, Msg: "ok", Key: key, Value: req.Value})
	default:
		// ignore unknown actions
	}
}

// sendManagementResp 统一补齐 response header 与 source_id，保证 management 回包链路稳定。
func (r *Runtime) sendManagementResp(req core.IHeader, action string, data any) {
	if r == nil || req == nil {
		return
	}
	body, err := transport.EncodeMessage(action, data)
	if err != nil {
		if r.log != nil {
			r.log.Warn("management encode failed", "err", err.Error())
		}
		return
	}
	selfID := r.AuthState().NodeID
	if selfID == 0 {
		selfID = req.TargetID()
	}
	respHdr := header.BuildTCPResponse(req, uint32(len(body)), protomgmt.SubProtoManagement)
	if respHdr == nil {
		return
	}
	if selfID != 0 {
		respHdr.WithSourceID(selfID)
	}
	if err := r.ensureClient().Send(respHdr, body); err != nil {
		r.storeLastError(err)
		if r.log != nil {
			r.log.Warn("management send failed", "action", action, "err", err.Error())
		}
	}
}
