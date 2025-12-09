package ziface

// IDataPack 解决TCP粘包问题的封包拆包模块(TLV)
type IDataPack interface {
	GetHeadLen() uint32                // 获取包头长度方法
	Pack(msg IMessage) ([]byte, error) // 封包
	Unpack([]byte) (IMessage, error)   // 拆包
}
