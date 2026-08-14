package knet

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"kinz/kiface"
)

func TestTLVPackRoundTrip(t *testing.T) {
	codec := NewTLVPack()
	wire, err := codec.Pack(NewMessage(42, []byte("hello kinz")))
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if len(wire) != 8+len("hello kinz") {
		t.Fatalf("wire len = %d, want %d", len(wire), 8+len("hello kinz"))
	}

	msgs, err := codec.Decode(wire)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("msgs = %d, want 1", len(msgs))
	}
	m := msgs[0]
	if m.GetMsgID() != 42 {
		t.Fatalf("MsgID = %d, want 42", m.GetMsgID())
	}
	if m.GetDataLen() != uint32(len("hello kinz")) {
		t.Fatalf("DataLen = %d, want %d", m.GetDataLen(), len("hello kinz"))
	}
	if !bytes.Equal(m.GetData(), []byte("hello kinz")) {
		t.Fatalf("payload = %q, want %q", m.GetData(), "hello kinz")
	}
}

func TestTLVPackHalfPacket(t *testing.T) {
	codec := NewTLVPack()
	wire, _ := codec.Pack(NewMessage(1, []byte("hello")))

	// partial header
	if msgs, err := codec.Decode(wire[:4]); err != nil || msgs != nil {
		t.Fatalf("Decode(partial header) = %v, %v; want nil, nil", msgs, err)
	}
	// header + partial body
	if msgs, err := codec.Decode(wire[4:10]); err != nil || msgs != nil {
		t.Fatalf("Decode(partial body) = %v, %v; want nil, nil", msgs, err)
	}
	// rest
	msgs, err := codec.Decode(wire[10:])
	if err != nil {
		t.Fatalf("Decode(rest): %v", err)
	}
	if len(msgs) != 1 || string(msgs[0].GetData()) != "hello" {
		t.Fatalf("msgs = %v, want [hello]", msgs)
	}
}

func TestTLVPackStickyPackets(t *testing.T) {
	codec := NewTLVPack()
	wire1, _ := codec.Pack(NewMessage(1, []byte("aa")))
	wire2, _ := codec.Pack(NewMessage(2, []byte("bb")))

	msgs, err := codec.Decode(append(wire1, wire2...))
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(msgs) != 2 || string(msgs[0].GetData()) != "aa" || string(msgs[1].GetData()) != "bb" {
		t.Fatalf("msgs = %v, want [aa bb]", msgs)
	}
	if msgs[0].GetMsgID() != 1 || msgs[1].GetMsgID() != 2 {
		t.Fatalf("msgIDs = %d %d, want 1 2", msgs[0].GetMsgID(), msgs[1].GetMsgID())
	}
}

func TestTLVPackPayloadIndependentOfBuffer(t *testing.T) {
	codec := NewTLVPack()
	wire1, _ := codec.Pack(NewMessage(1, []byte("first-message-payload")))
	wire2, _ := codec.Pack(NewMessage(2, []byte("second")))

	msgs, err := codec.Decode(wire1)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	first := msgs[0]
	// A subsequent Decode must NOT corrupt the previous message's payload.
	if _, err := codec.Decode(wire2); err != nil {
		t.Fatalf("second Decode: %v", err)
	}
	if string(first.GetData()) != "first-message-payload" {
		t.Fatalf("first payload corrupted: %q", first.GetData())
	}
}

func TestTLVPackRejectsOversize(t *testing.T) {
	codec := NewTLVPackWithOrder(binary.LittleEndian, 4096)
	// DataLen = 0x00100000 (little-endian), MsgID = 1.
	head := []byte{0x00, 0x00, 0x10, 0x00, 0x01, 0x00, 0x00, 0x00}
	if _, err := codec.Decode(head); err == nil {
		t.Fatal("expected error for oversize packet")
	} else if !errors.Is(err, kiface.ErrTooLargePacket) {
		t.Fatalf("err = %v, want ErrTooLargePacket", err)
	}
}

func TestTLVPackBigEndian(t *testing.T) {
	codec := NewTLVPackWithOrder(binary.BigEndian, 4096)
	wire, err := codec.Pack(NewMessage(0x0102, []byte("be")))
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	// Big-endian header: DataLen=2 as 00 00 00 02, MsgID=0x0102 as 00 00 01 02.
	if wire[3] != 0x02 {
		t.Fatalf("DataLen high byte = %d, want 2", wire[3])
	}
	if wire[6] != 0x01 || wire[7] != 0x02 {
		t.Fatalf("MsgID bytes = %x %x, want 01 02", wire[6], wire[7])
	}

	msgs, err := codec.Decode(wire)
	if err != nil {
		t.Fatalf("Decode: %v", err)
	}
	if len(msgs) != 1 || msgs[0].GetMsgID() != 0x0102 {
		t.Fatalf("msgs = %v, want MsgID 0x0102", msgs)
	}
}

func TestTLVPackCloneIndependent(t *testing.T) {
	tpl := NewTLVPack()
	a := tpl.Clone()
	b := tpl.Clone()

	wire, _ := tpl.Pack(NewMessage(1, []byte("x")))
	if _, err := a.Decode(wire[:4]); err != nil { // a buffers partial data
		t.Fatalf("a.Decode: %v", err)
	}
	msgs, err := b.Decode(wire) // b is unaffected by a's state
	if err != nil {
		t.Fatalf("b.Decode: %v", err)
	}
	if len(msgs) != 1 {
		t.Fatalf("b msgs = %d, want 1", len(msgs))
	}
}

func TestMessageSetters(t *testing.T) {
	msg := NewMessage(1, nil)
	msg.SetMsgID(7)
	msg.SetData([]byte("xyz"))
	msg.SetDataLen(2)

	if msg.GetMsgID() != 7 {
		t.Fatalf("MsgID = %d, want 7", msg.GetMsgID())
	}
	if msg.GetDataLen() != 2 {
		t.Fatalf("DataLen = %d, want 2", msg.GetDataLen())
	}
	if !bytes.Equal(msg.GetData(), []byte("xyz")) {
		t.Fatalf("Data = %q, want xyz", msg.GetData())
	}
	if msg.GetRawData() != nil {
		t.Fatal("Raw should be nil for a fresh message")
	}
}
