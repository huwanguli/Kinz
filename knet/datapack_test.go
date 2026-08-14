package knet

import (
	"bytes"
	"errors"
	"testing"

	"kinz/kiface"
)

func TestDataPackRoundTrip(t *testing.T) {
	dp := NewDataPack()
	msg := NewMessage(42, []byte("hello kinz"))

	wire, err := dp.Pack(msg)
	if err != nil {
		t.Fatalf("Pack: %v", err)
	}
	if len(wire) != int(dp.GetHeadLen())+len("hello kinz") {
		t.Fatalf("wire len = %d, want %d", len(wire), dp.GetHeadLen()+uint32(len("hello kinz")))
	}

	head := wire[:dp.GetHeadLen()]
	unpacked, err := dp.Unpack(head)
	if err != nil {
		t.Fatalf("Unpack: %v", err)
	}
	if unpacked.GetMsgID() != 42 {
		t.Fatalf("MsgID = %d, want 42", unpacked.GetMsgID())
	}
	if unpacked.GetDataLen() != uint32(len("hello kinz")) {
		t.Fatalf("DataLen = %d, want %d", unpacked.GetDataLen(), len("hello kinz"))
	}
	if !bytes.Equal(unpacked.GetRawData(), head) {
		t.Fatalf("Raw = %q, want head %q", unpacked.GetRawData(), head)
	}
	if !bytes.Equal(wire[dp.GetHeadLen():], []byte("hello kinz")) {
		t.Fatalf("payload = %q, want %q", wire[dp.GetHeadLen():], "hello kinz")
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

func TestDataPackRejectsOversize(t *testing.T) {
	// DataLen = 0x00100000 (little-endian), MsgID = 1.
	head := []byte{0x00, 0x00, 0x10, 0x00, 0x01, 0x00, 0x00, 0x00}
	if _, err := NewDataPack().Unpack(head); err == nil {
		t.Fatal("expected error for oversize packet")
	} else if !errors.Is(err, kiface.ErrTooLargePacket) {
		t.Fatalf("err = %v, want ErrTooLargePacket", err)
	}
}

func TestDataPackRejectsShortHeader(t *testing.T) {
	if _, err := NewDataPack().Unpack([]byte{0x01}); err == nil {
		t.Fatal("expected error for short header")
	}
}
