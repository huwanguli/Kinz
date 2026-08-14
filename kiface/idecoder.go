package kiface

// IDecoder converts a raw byte stream into complete frames, handling TCP
// sticky and half packets. Stateful: a single instance serves one connection;
// use Clone to obtain a per-connection instance from a template.
type IDecoder interface {
	// Decode consumes buffered bytes and returns complete frames.
	// It returns (nil, nil) when more data is needed for a full frame.
	// Protocol violations return an error (e.g. ErrTooLargePacket, ErrProtocol).
	Decode(buff []byte) ([][]byte, error)
	// Clone returns an independent decoder with the same configuration,
	// ready to serve a new connection.
	Clone() IDecoder
	// GetLengthField returns the length-field configuration of this decoder.
	GetLengthField() *LengthField
}
