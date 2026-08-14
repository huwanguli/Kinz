package kiface

// IDecoder converts a raw byte stream into complete frames, handling TCP
// sticky and half packets. Stateful: a single instance serves one connection.
type IDecoder interface {
	// Decode consumes buffered bytes and returns complete frames.
	// It returns (nil, nil) when more data is needed for a full frame.
	// Protocol errors are returned as errors (Phase 2 converts FrameDecoder).
	Decode(buff []byte) [][]byte
	// GetLengthField returns the length-field configuration of this decoder.
	GetLengthField() *LengthField
}
