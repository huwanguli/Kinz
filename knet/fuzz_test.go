package knet

import (
	"testing"
)

// FuzzTLVPackDecode feeds arbitrary byte streams to the TLV codec. Decode must
// never panic or crash, regardless of how malformed the input is: random
// header lengths, negative-style huge lengths (capped by maxPacketSize) and
// half/sticky frames all have defined behavior.
//
// Run with: go test ./knet/ -fuzz=FuzzTLVPackDecode -fuzztime=30s
func FuzzTLVPackDecode(f *testing.F) {
	// Seed corpus: a valid frame, a half frame, a sticky pair, and garbage.
	f.Add([]byte{0x02, 0, 0, 0, 0x01, 0, 0, 0, 'h', 'i'})
	f.Add([]byte{0x02, 0, 0, 0})                // header only
	f.Add([]byte{0x02, 0, 0, 0, 0x01, 0, 0, 0}) // header + missing body
	f.Add([]byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})
	f.Add([]byte("not a tlv frame at all, just some random stream bytes"))
	f.Add([]byte{0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})

	f.Fuzz(func(t *testing.T, data []byte) {
		codec := NewTLVPack()
		msgs, err := codec.Decode(data)
		if err == nil {
			// Every reported message must be self-consistent.
			for _, m := range msgs {
				if int(m.GetDataLen()) != len(m.GetData()) {
					t.Fatalf("msg dataLen %d != len(data) %d", m.GetDataLen(), len(m.GetData()))
				}
			}
		}
	})
}
