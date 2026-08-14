package knet

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"kinz/kiface"
)

// defaultMaxPacketSize caps a single packet payload when no config is set.
const defaultMaxPacketSize uint32 = 4096

// TLVPack is the default ICodec: the TLV wire format
// [DataLen:4][MsgID:4][Data:DataLen] in a configurable byte order
// (little-endian by default). It is stateful (buffers partial frames for TCP
// sticky/half packets); use Clone for a per-connection instance.
type TLVPack struct {
	order         binary.ByteOrder
	maxPacketSize uint32
	in            []byte
}

// NewTLVPack returns a little-endian TLVPack with the default max packet size.
func NewTLVPack() *TLVPack {
	return NewTLVPackWithOrder(binary.LittleEndian, defaultMaxPacketSize)
}

// NewTLVPackWithOrder returns a TLVPack with the given byte order and max
// packet size (maxPacketSize <= 0 means unlimited).
func NewTLVPackWithOrder(order binary.ByteOrder, maxPacketSize uint32) *TLVPack {
	if order == nil {
		order = binary.LittleEndian
	}
	return &TLVPack{order: order, maxPacketSize: maxPacketSize}
}

// Decode implements kiface.ICodec: consumes raw stream bytes and returns
// complete TLV messages. Returned payloads are independent of the codec's
// internal buffer, so they stay valid for asynchronous processing.
func (t *TLVPack) Decode(buff []byte) ([]kiface.IMessage, error) {
	t.in = append(t.in, buff...)
	var msgs []kiface.IMessage
	for {
		if len(t.in) < 8 {
			return msgs, nil // header incomplete
		}
		dataLen := t.order.Uint32(t.in[0:4])
		msgID := t.order.Uint32(t.in[4:8])
		if t.maxPacketSize > 0 && dataLen > t.maxPacketSize {
			return msgs, fmt.Errorf("%w: got %d bytes, max %d",
				kiface.ErrTooLargePacket, dataLen, t.maxPacketSize)
		}
		total := 8 + int(dataLen)
		if len(t.in) < total {
			return msgs, nil // frame body incomplete (half packet)
		}
		msg := &Message{
			Id:      msgID,
			DataLen: dataLen,
			// Copy payload: t.in will be overwritten by the next Decode.
			Data: append([]byte(nil), t.in[8:total]...),
			Raw:  append([]byte(nil), t.in[:8]...),
		}
		msgs = append(msgs, msg)
		t.in = t.in[total:]
	}
}

// Pack implements kiface.ICodec: serializes msg into TLV wire bytes.
func (t *TLVPack) Pack(msg kiface.IMessage) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, 8+msg.GetDataLen()))
	if err := binary.Write(buf, t.order, msg.GetDataLen()); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, t.order, msg.GetMsgID()); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, t.order, msg.GetData()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Clone implements kiface.ICodec: an independent codec with the same
// configuration, ready to serve a new connection.
func (t *TLVPack) Clone() kiface.ICodec {
	return &TLVPack{order: t.order, maxPacketSize: t.maxPacketSize}
}
