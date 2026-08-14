package kinterceptor

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"kinz/kiface"
)

// bigLen2 builds a frame with a 2-byte big-endian length prefix.
func bigLen2(payload []byte) []byte {
	frame := make([]byte, 2+len(payload))
	binary.BigEndian.PutUint16(frame, uint16(len(payload)))
	copy(frame[2:], payload)
	return frame
}

func newBigLen2Decoder(max uint64) *FrameDecoder {
	return NewFrameDecoder(kiface.LengthField{
		Order:               binary.BigEndian,
		MaxFrameLength:      max,
		LengthFieldOffset:   0,
		LengthFieldLength:   2,
		InitialBytesToStrip: 2,
	})
}

func TestDecodeSingleFrame(t *testing.T) {
	d := newBigLen2Decoder(1024)
	if d.GetLengthField().MaxFrameLength != 1024 {
		t.Fatalf("GetLengthField().MaxFrameLength = %d, want 1024", d.GetLengthField().MaxFrameLength)
	}
	frames, err := d.Decode(bigLen2([]byte("hello")))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(frames) != 1 || string(frames[0]) != "hello" {
		t.Fatalf("frames = %q, want [hello]", frames)
	}
}

func TestDecodeHalfPacket(t *testing.T) {
	d := newBigLen2Decoder(1024)
	wire := bigLen2([]byte("hello"))

	// header only -> no frame yet
	frames, err := d.Decode(wire[:2])
	if err != nil || frames != nil {
		t.Fatalf("Decode(header) = %v, %v; want nil, nil", frames, err)
	}
	// header + partial body -> still no frame
	frames, err = d.Decode(wire[2:4])
	if err != nil || frames != nil {
		t.Fatalf("Decode(partial) = %v, %v; want nil, nil", frames, err)
	}
	// rest -> one frame
	frames, err = d.Decode(wire[4:])
	if err != nil {
		t.Fatalf("Decode(rest): %v", err)
	}
	if len(frames) != 1 || string(frames[0]) != "hello" {
		t.Fatalf("frames = %q, want [hello]", frames)
	}
}

func TestDecodeStickyPackets(t *testing.T) {
	d := newBigLen2Decoder(1024)
	wire := append(bigLen2([]byte("aa")), bigLen2([]byte("bb"))...)
	frames, err := d.Decode(wire)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(frames) != 2 || string(frames[0]) != "aa" || string(frames[1]) != "bb" {
		t.Fatalf("frames = %q, want [aa bb]", frames)
	}
}

func TestDecodeInitialBytesToStrip(t *testing.T) {
	d := NewFrameDecoder(kiface.LengthField{
		Order:               binary.BigEndian,
		MaxFrameLength:      1024,
		LengthFieldOffset:   0,
		LengthFieldLength:   2,
		InitialBytesToStrip: 2,
	})
	wire := bigLen2([]byte("strip-me"))
	frames, err := d.Decode(wire)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(frames) != 1 || string(frames[0]) != "strip-me" {
		t.Fatalf("frames = %q, want [strip-me]", frames)
	}
}

func TestDecodeTooLong(t *testing.T) {
	d := newBigLen2Decoder(8)
	_, err := d.Decode(bigLen2([]byte("1234567890")))
	if err == nil {
		t.Fatal("expected error for oversized frame")
	} else if !errors.Is(err, kiface.ErrTooLargePacket) {
		t.Fatalf("err = %v, want ErrTooLargePacket", err)
	}
}

func TestDecodeNegativeLength(t *testing.T) {
	d := NewFrameDecoder(kiface.LengthField{
		Order:             binary.BigEndian,
		MaxFrameLength:    0, // unlimited
		LengthFieldOffset: 0,
		LengthFieldLength: 8,
	})
	// 8 bytes of 0xFF -> uint64 max -> int64 = -1.
	wire := bytes.Repeat([]byte{0xFF}, 8)
	if _, err := d.Decode(wire); err == nil {
		t.Fatal("expected error for negative length field")
	} else if !errors.Is(err, kiface.ErrProtocol) {
		t.Fatalf("err = %v, want ErrProtocol", err)
	}
}

func TestDecodeUnsupportedLengthFieldWidth(t *testing.T) {
	d := NewFrameDecoder(kiface.LengthField{
		Order:             binary.BigEndian,
		MaxFrameLength:    1024,
		LengthFieldOffset: 0,
		LengthFieldLength: 5,
	})
	wire := make([]byte, 5)
	if _, err := d.Decode(wire); err == nil {
		t.Fatal("expected error for unsupported width")
	} else if !errors.Is(err, kiface.ErrProtocol) {
		t.Fatalf("err = %v, want ErrProtocol", err)
	}
}

func TestDecodeLengthFieldWidths(t *testing.T) {
	cases := []struct {
		width  int
		encode func(v int, payload []byte) []byte
	}{
		{1, func(v int, p []byte) []byte {
			return append([]byte{byte(v)}, p...)
		}},
		{3, func(v int, p []byte) []byte {
			return append([]byte{byte(v >> 16), byte(v >> 8), byte(v)}, p...)
		}},
		{4, func(v int, p []byte) []byte {
			f := make([]byte, 4+len(p))
			binary.BigEndian.PutUint32(f, uint32(v))
			copy(f[4:], p)
			return f
		}},
	}
	for _, tc := range cases {
		d := NewFrameDecoder(kiface.LengthField{
			Order:               binary.BigEndian,
			MaxFrameLength:      1024,
			LengthFieldOffset:   0,
			LengthFieldLength:   tc.width,
			InitialBytesToStrip: tc.width,
		})
		payload := []byte("payload")
		frames, err := d.Decode(tc.encode(len(payload), payload))
		if err != nil {
			t.Fatalf("width %d: Decode: %v", tc.width, err)
		}
		if len(frames) != 1 || string(frames[0]) != "payload" {
			t.Fatalf("width %d: frames = %q, want [payload]", tc.width, frames)
		}
	}
}

func TestDecodeLittleEndian(t *testing.T) {
	d := NewFrameDecoder(kiface.LengthField{
		Order:               binary.LittleEndian,
		MaxFrameLength:      1024,
		LengthFieldOffset:   0,
		LengthFieldLength:   4,
		InitialBytesToStrip: 4,
	})
	payload := []byte("le")
	frame := make([]byte, 4+len(payload))
	binary.LittleEndian.PutUint32(frame, uint32(len(payload)))
	copy(frame[4:], payload)

	frames, err := d.Decode(frame)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(frames) != 1 || string(frames[0]) != "le" {
		t.Fatalf("frames = %q, want [le]", frames)
	}
}

func TestDecodeStripTooLarge(t *testing.T) {
	d := NewFrameDecoder(kiface.LengthField{
		Order:               binary.BigEndian,
		MaxFrameLength:      1024,
		LengthFieldOffset:   0,
		LengthFieldLength:   2,
		InitialBytesToStrip: 100,
	})
	if _, err := d.Decode(bigLen2([]byte("x"))); err == nil {
		t.Fatal("expected error when initialBytesToStrip exceeds frame length")
	} else if !errors.Is(err, kiface.ErrProtocol) {
		t.Fatalf("err = %v, want ErrProtocol", err)
	}
}

func TestCloneIndependentState(t *testing.T) {
	tpl := newBigLen2Decoder(1024)
	a := tpl.Clone()
	b := tpl.Clone()

	wire := bigLen2([]byte("x"))
	if _, err := a.Decode(wire[:2]); err != nil { // a accumulates half data
		t.Fatalf("a.Decode: %v", err)
	}
	// b is unaffected by a's state
	frames, err := b.Decode(wire)
	if err != nil {
		t.Fatalf("b.Decode: %v", err)
	}
	if len(frames) != 1 || string(frames[0]) != "x" {
		t.Fatalf("b frames = %q, want [x]", frames)
	}
}
