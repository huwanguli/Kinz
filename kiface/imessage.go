package kiface

// IMessage is a single protocol message (payload container).
type IMessage interface {
	// GetMsgID returns the message id.
	GetMsgID() uint32
	// GetDataLen returns the payload length in bytes.
	GetDataLen() uint32
	// GetData returns the payload.
	GetData() []byte
	// GetRawData returns the raw bytes seen by the decoder (header when unpacked).
	GetRawData() []byte
	// SetMsgID sets the message id.
	SetMsgID(uint32)
	// SetData sets the payload.
	SetData([]byte)
	// SetDataLen sets the payload length.
	SetDataLen(uint32)
}
