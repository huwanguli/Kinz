package kiface

type IConnManager interface {
	// Add 添加链接
	Add(conn IConnection)
	// Remove 删除链接
	Remove(conn IConnection)
	// Get 根据ID获取链接
	Get(connID uint64) (IConnection, error)
	Get2(string) (IConnection, error)
	// Len 得到当前链接总数
	Len() int
	// ClearConn 清除并终止所有链接
	ClearConn()
	GetAllConnID() []uint64
	GetAllConnIdStr() []string
	Range(func(uint64, IConnection, interface{}) error, interface{}) error
	Range2(func(string, IConnection, interface{}) error, interface{}) error
}
