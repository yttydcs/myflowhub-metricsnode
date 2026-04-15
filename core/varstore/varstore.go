package varstore

// 本文件承载 MetricsNode 应用层中与 `varstore` 相关的逻辑。

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	core "github.com/yttydcs/myflowhub-core"
	"github.com/yttydcs/myflowhub-core/header"
	protovar "github.com/yttydcs/myflowhub-proto/protocol/varstore"
	"github.com/yttydcs/myflowhub-sdk/transport"
)

type Sender interface {
	Send(hdr core.IHeader, payload []byte) error
}

type Client struct {
	sender Sender
	log    *slog.Logger
}

func New(sender Sender, log *slog.Logger) *Client {
	if log == nil {
		log = slog.Default()
	}
	return &Client{sender: sender, log: log}
}

func (c *Client) Set(sourceID, targetID uint32, req protovar.SetReq) error {
	if c == nil || c.sender == nil {
		return errors.New("varstore client not initialized")
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Value = strings.TrimSpace(req.Value)
	req.Visibility = strings.TrimSpace(req.Visibility)

	if !ValidVarName(req.Name) {
		return fmt.Errorf("invalid var name: %q", req.Name)
	}
	if req.Value == "" {
		return errors.New("value is required")
	}
	if req.Visibility == "" {
		req.Visibility = protovar.VisibilityPublic
	}

	payload, err := transport.EncodeMessage(protovar.ActionSet, req)
	if err != nil {
		return err
	}
	hdr := (&header.HeaderTcp{}).
		WithMajor(header.MajorCmd).
		WithSubProto(protovar.SubProtoVarStore).
		WithSourceID(sourceID).
		WithTargetID(targetID).
		WithMsgID(uint32(time.Now().UnixNano())).
		WithTimestamp(uint32(time.Now().Unix()))
	if err := c.sender.Send(hdr, payload); err != nil {
		return err
	}
	if c.log != nil {
		c.log.Debug("varstore set sent", "name", req.Name, "owner", req.Owner, "visibility", req.Visibility)
	}
	return nil
}

func ValidVarName(name string) bool {
	if name == "" {
		return false
	}
	for i := 0; i < len(name); i++ {
		ch := name[i]
		if ch >= 'a' && ch <= 'z' {
			continue
		}
		if ch >= 'A' && ch <= 'Z' {
			continue
		}
		if ch >= '0' && ch <= '9' {
			continue
		}
		if ch == '_' {
			continue
		}
		return false
	}
	return true
}
