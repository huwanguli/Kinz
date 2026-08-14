package kiface

// ICodec is the wire codec seam: framing (TCP sticky/half packets) and message
// serialization in a single unit. A protocol implementation owns both concerns,
// so a custom wire format is ONE interface instead of two coupled ones.
//
// Implementations are stateful (they buffer partial frames for framing) and
// must return messages whose payloads are independent of the codec's internal
// buffer — messages may be processed asynchronously after Decode returns.
// Use Clone to obtain a per-connection instance from a template.
type ICodec interface {
	// Decode consumes raw stream bytes and returns complete messages.
	// It returns (nil, nil) when more data is needed for a full frame.
	// Protocol violations return an error (e.g. ErrTooLargePacket, ErrProtocol).
	Decode(buff []byte) ([]IMessage, error)
	// Pack serializes a message into wire-format bytes.
	Pack(msg IMessage) ([]byte, error)
	// Clone returns an independent codec with the same configuration,
	// ready to serve a new connection.
	Clone() ICodec
}
