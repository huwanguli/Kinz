package kiface

// IDataPack packs and unpacks messages in a TLV wire format.
type IDataPack interface {
	// GetHeadLen returns the header length in bytes.
	GetHeadLen() uint32
	// Pack serializes msg into wire-format bytes.
	Pack(msg IMessage) ([]byte, error)
	// Unpack parses the header from binaryData and returns a message with the
	// id and data length set; the payload must be read separately.
	Unpack(binaryData []byte) (IMessage, error)
}
