package ziface

// IRequest 将客户端请求的链接信息和请求数据绑定在一起
type IRequest interface {
	// GetConnection 得到当前链接数据
	GetConnection() IConnection

	// GetData 得到当前数据
	GetData() []byte
}
