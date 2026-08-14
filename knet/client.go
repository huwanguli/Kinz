package knet

import (
	"time"

	"kinz/kiface"
)

// Client implements kiface.IClient. Behavioral implementation lands in Phase 3.
type Client struct {
	name string
}

// NewClient creates a Client and applies opts. Full client lands in Phase 3.
func NewClient(ip string, port int, opts ...ClientOption) kiface.IClient {
	c := &Client{name: "KinzClient"}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Start implements kiface.IClient. Phase 3.
func (c *Client) Start() error { return kiface.ErrNotImplemented }

// Stop implements kiface.IClient. Phase 3.
func (c *Client) Stop() {}

// Restart implements kiface.IClient. Phase 3.
func (c *Client) Restart() {}

// Conn implements kiface.IClient. Phase 3.
func (c *Client) Conn() kiface.IConnection { return nil }

// AddRouterSlices implements kiface.IClient. Phase 3.
func (c *Client) AddRouterSlices(msgID uint32, handlers ...kiface.RouterHandler) (kiface.IRouterSlices, error) {
	return nil, kiface.ErrNotImplemented
}

// SetOnConnStart implements kiface.IClient. Phase 3.
func (c *Client) SetOnConnStart(f func(kiface.IConnection)) {}

// SetOnConnStop implements kiface.IClient. Phase 3.
func (c *Client) SetOnConnStop(f func(kiface.IConnection)) {}

// GetOnConnStart implements kiface.IClient.
func (c *Client) GetOnConnStart() func(kiface.IConnection) { return nil }

// GetOnConnStop implements kiface.IClient.
func (c *Client) GetOnConnStop() func(kiface.IConnection) { return nil }

// SetCodec implements kiface.IClient.
func (c *Client) SetCodec(codec kiface.ICodec) {}

// GetCodec implements kiface.IClient.
func (c *Client) GetCodec() kiface.ICodec { return nil }

// GetMsgHandler implements kiface.IClient. Phase 3.
func (c *Client) GetMsgHandler() kiface.IMsgHandle { return nil }

// StartHeartBeat implements kiface.IClient. Phase 3.
func (c *Client) StartHeartBeat(interval time.Duration) {}

// StartHeartBeatWithOption implements kiface.IClient. Phase 3.
func (c *Client) StartHeartBeatWithOption(interval time.Duration, option *kiface.HeartBeatOption) {}

// SetName implements kiface.IClient.
func (c *Client) SetName(name string) { c.name = name }

// GetName implements kiface.IClient.
func (c *Client) GetName() string { return c.name }
