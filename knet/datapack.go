package knet

import (
	"bytes"
	"encoding/binary"
	"fmt"

	"kinz/kiface"
)

// defaultMaxPacketSize caps a single packet payload when no config is set
// (matches the historical default; Phase 2 wires it to kconf).
const defaultMaxPacketSize uint32 = 4096

// DataPack implements the default TLV wire format:
// [DataLen:4][MsgID:4][Data:DataLen], in the configured byte order
// (little-endian by default for wire compatibility with the legacy protocol).
type DataPack struct {
	order         binary.ByteOrder
	maxPacketSize uint32
}

// NewDataPack returns a DataPack with little-endian order and the default
// max packet size.
func NewDataPack() *DataPack {
	return NewDataPackWithOrder(binary.LittleEndian)
}

// NewDataPackWithOrder returns a DataPack using the given byte order.
// A nil order falls back to little-endian.
func NewDataPackWithOrder(order binary.ByteOrder) *DataPack {
	if order == nil {
		order = binary.LittleEndian
	}
	return &DataPack{order: order, maxPacketSize: defaultMaxPacketSize}
}

// GetHeadLen returns the header length in bytes.
func (dp *DataPack) GetHeadLen() uint32 { return 8 }

// Pack serializes msg into wire-format bytes.
func (dp *DataPack) Pack(msg kiface.IMessage) ([]byte, error) {
	buf := bytes.NewBuffer(make([]byte, 0, dp.GetHeadLen()+msg.GetDataLen()))
	if err := binary.Write(buf, dp.order, msg.GetDataLen()); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, dp.order, msg.GetMsgID()); err != nil {
		return nil, err
	}
	if err := binary.Write(buf, dp.order, msg.GetData()); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Unpack parses the header and returns a Message with id/dataLen set;
// the payload must be read separately (see Phase 2 pipeline).
func (dp *DataPack) Unpack(binaryData []byte) (kiface.IMessage, error) {
	if len(binaryData) < int(dp.GetHeadLen()) {
		return nil, fmt.Errorf("%w: header needs %d bytes, got %d",
			kiface.ErrProtocol, dp.GetHeadLen(), len(binaryData))
	}
	reader := bytes.NewReader(binaryData[:dp.GetHeadLen()])
	msg := &Message{}
	if err := binary.Read(reader, dp.order, &msg.DataLen); err != nil {
		return nil, err
	}
	if err := binary.Read(reader, dp.order, &msg.Id); err != nil {
		return nil, err
	}
	if dp.maxPacketSize > 0 && msg.DataLen > dp.maxPacketSize {
		return nil, fmt.Errorf("%w: got %d bytes, max %d",
			kiface.ErrTooLargePacket, msg.DataLen, dp.maxPacketSize)
	}
	msg.Raw = append([]byte(nil), binaryData[:dp.GetHeadLen()]...)
	return msg, nil
}
