package kiface

// IDataPack 解决TCP粘包问题的封包拆包模块(TLV)
type IDataPack interface {
	GetHeadLen() uint32                // 获取包头长度方法
	Pack(msg IMessage) ([]byte, error) // 封包
	Unpack([]byte) (IMessage, error)   // 拆包
}

const (
	// ZinxDataPack 大端序
	ZinxDataPack string = "zinx_pack_tlv_big_endian"
	// ZinxDataPackOld 小端序
	ZinxDataPackOld string = "zinx_pack_tlv_little_endian"
)

const (
	ZinxMessage string = "zinx_message"
)
