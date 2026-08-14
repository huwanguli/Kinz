package knet

// Message is the default IMessage implementation: a TLV payload container.
// Fields are exported for direct decoder use (binary.Read needs field pointers).
type Message struct {
	Id      uint32
	DataLen uint32
	Data    []byte
	Raw     []byte
}

// NewMessage creates a Message with id and payload (DataLen derived).
func NewMessage(id uint32, data []byte) *Message {
	return &Message{Id: id, DataLen: uint32(len(data)), Data: data}
}

// GetMsgID returns the message id.
func (m *Message) GetMsgID() uint32 { return m.Id }

// GetDataLen returns the payload length.
func (m *Message) GetDataLen() uint32 { return m.DataLen }

// GetData returns the payload.
func (m *Message) GetData() []byte { return m.Data }

// GetRawData returns the raw header bytes captured during Unpack.
func (m *Message) GetRawData() []byte { return m.Raw }

// SetMsgID sets the message id.
func (m *Message) SetMsgID(id uint32) { m.Id = id }

// SetData sets the payload (DataLen is set by the decoder from the header).
func (m *Message) SetData(data []byte) { m.Data = data }

// SetDataLen sets the payload length.
func (m *Message) SetDataLen(l uint32) { m.DataLen = l }
