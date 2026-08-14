package kiface

import "encoding/binary"

type IFrameDecoder interface {
	Decode(buff []byte) [][]byte
}

type LengthField struct {
	Order               binary.ByteOrder //大小端
	MaxFrameLength      uint64           // 最大帧长度
	LengthFieldOffset   int              // 长度字段偏移量
	LengthFieldLength   int              // 长度域字段的字节数
	LengthAdjustment    int              // 长度调整
	InitialBytesToStrip int              //需要跳过的字节数
}
