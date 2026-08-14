package knet

import "kinz/kiface"

// ClientOption customizes a Client at construction time.
type ClientOption func(c kiface.IClient)

// WithCodecClient sets the wire codec (framing + serialization).
func WithCodecClient(codec kiface.ICodec) ClientOption {
	return func(c kiface.IClient) {
		c.SetCodec(codec)
	}
}

// WithNameClient sets the client name.
func WithNameClient(name string) ClientOption {
	return func(c kiface.IClient) {
		c.SetName(name)
	}
}
