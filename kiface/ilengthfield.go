package kiface

import "encoding/binary"

// LengthField describes how to locate the frame length in a byte stream.
type LengthField struct {
	// Order is the byte order of the length field (default big-endian).
	Order binary.ByteOrder
	// MaxFrameLength caps a single frame; larger frames are treated as errors.
	MaxFrameLength uint64
	// LengthFieldOffset is the offset of the length field in the frame.
	LengthFieldOffset int
	// LengthFieldLength is the byte width of the length field (1/2/3/4/8).
	LengthFieldLength int
	// LengthAdjustment is added to the length-field value.
	LengthAdjustment int
	// InitialBytesToStrip is how many leading bytes to drop from each frame.
	InitialBytesToStrip int
}
