package ziface

type IConnManager interface {
	// Add 添加链接
	Add(conn IConnection)
	// Remove 删除链接
	Remove(conn IConnection)
	// Get 根据ID获取链接
	Get(connID uint32) (IConnection, error)
	// Len 得到当前链接总数
	Len() int
	// ClearConn 清除并终止所有链接
	ClearConn()
}
