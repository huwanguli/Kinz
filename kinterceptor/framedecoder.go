package kinterceptor

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"kinz/kiface"
)

// FrameDecoder is a LengthField-based frame decoder (netty-style) that turns a
// raw TCP byte stream into complete frames, handling sticky and half packets.
// It is stateful: one instance per connection (see Clone).
//
// Configuration (LengthField): the frame layout is
//
//	[LengthFieldOffset bytes][LengthFieldLength-byte length][...][Data]
//
// and the total frame length equals lengthFieldValue + LengthAdjustment +
// LengthFieldEndOffset. InitialBytesToStrip leading bytes are removed from
// each returned frame.
//
// Protocol violations (negative length, frame exceeding MaxFrameLength,
// unsupported length-field width) are returned as errors; the caller should
// close the connection (fail-fast posture). MaxFrameLength == 0 means no limit.
type FrameDecoder struct {
	kiface.LengthField

	LengthFieldEndOffset int
	in                   []byte
}

// NewFrameDecoder creates a FrameDecoder from a length-field configuration.
func NewFrameDecoder(lf kiface.LengthField) *FrameDecoder {
	d := &FrameDecoder{
		LengthField:          lf,
		LengthFieldEndOffset: lf.LengthFieldOffset + lf.LengthFieldLength,
		in:                   make([]byte, 0),
	}
	if d.Order == nil {
		d.Order = binary.BigEndian
	}
	return d
}

// Clone implements kiface.IDecoder: an independent decoder with the same
// configuration, ready to serve a new connection.
func (d *FrameDecoder) Clone() kiface.IDecoder {
	return NewFrameDecoder(d.LengthField)
}

// GetLengthField returns the length-field configuration of this decoder.
func (d *FrameDecoder) GetLengthField() *kiface.LengthField {
	return &d.LengthField
}

// Decode implements kiface.IDecoder. It consumes buffered bytes and returns
// complete frames; (nil, nil) means more data is needed for a full frame.
func (d *FrameDecoder) Decode(buff []byte) ([][]byte, error) {
	d.in = append(d.in, buff...)
	var frames [][]byte
	for {
		frame, err := d.decode(d.in)
		if err != nil {
			return frames, err
		}
		if frame == nil {
			return frames, nil
		}
		frames = append(frames, frame)
		consumed := len(frame) + d.InitialBytesToStrip
		d.in = d.in[consumed:]
	}
}

// decode attempts to extract one frame from buf.
func (d *FrameDecoder) decode(buf []byte) ([]byte, error) {
	in := bytes.NewBuffer(buf)
	if in.Len() < d.LengthFieldEndOffset {
		return nil, nil // header (including the length field) is incomplete
	}

	rawLen, err := d.getUnadjustedFrameLength(
		in.Bytes(), d.LengthFieldOffset, d.LengthFieldLength, d.Order)
	if err != nil {
		return nil, err
	}
	if rawLen < 0 {
		return nil, fmt.Errorf("%w: negative pre-adjustment length field: %d",
			kiface.ErrProtocol, rawLen)
	}

	frameLength := rawLen + int64(d.LengthAdjustment) + int64(d.LengthFieldEndOffset)
	if frameLength < int64(d.InitialBytesToStrip) {
		return nil, fmt.Errorf("%w: adjusted frame length %d is less than initial bytes to strip %d",
			kiface.ErrProtocol, frameLength, d.InitialBytesToStrip)
	}
	if d.MaxFrameLength > 0 && uint64(frameLength) > d.MaxFrameLength {
		return nil, fmt.Errorf("%w: frame length %d exceeds max %d",
			kiface.ErrTooLargePacket, frameLength, d.MaxFrameLength)
	}
	if in.Len() < int(frameLength) {
		return nil, nil // frame body is incomplete (half packet)
	}

	in.Next(d.InitialBytesToStrip)
	out := make([]byte, int(frameLength)-d.InitialBytesToStrip)
	_, _ = in.Read(out)
	return out, nil
}

// getUnadjustedFrameLength reads the raw length-field value (before
// LengthAdjustment is applied).
func (d *FrameDecoder) getUnadjustedFrameLength(buf []byte, offset, length int, order binary.ByteOrder) (int64, error) {
	field := buf[offset : offset+length]
	switch length {
	case 1:
		return int64(field[0]), nil
	case 2:
		return int64(order.Uint16(field)), nil
	case 3:
		if order == binary.LittleEndian {
			return int64(uint(field[0]) | uint(field[1])<<8 | uint(field[2])<<16), nil
		}
		return int64(uint(field[2]) | uint(field[1])<<8 | uint(field[0])<<16), nil
	case 4:
		return int64(order.Uint32(field)), nil
	case 8:
		return int64(order.Uint64(field)), nil
	default:
		return 0, fmt.Errorf("%w: unsupported LengthFieldLength %d (want 1/2/3/4/8)",
			kiface.ErrProtocol, length)
	}
}
