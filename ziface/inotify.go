package ziface

type Inotify interface {
	HasIdConn(id uint64) bool

	ConnNums() int
	// SetNotifyID 添加连接
	SetNotifyID(id uint64, conn IConnection)

	GetNotifyByID(id uint64) (IConnection, error)

	DelNotifyByID(id uint64) error

	NotifyToConnByID(id uint64, MsgId uint32, data []byte) error

	NotifyAll(MsgId uint32, data []byte) error //通知所有人
	// NotifyBuffToConnByID 用缓冲队列通知某个 ID 的方法
	NotifyBuffToConnByID(Id uint64, MsgId uint32, data []byte) error
	// NotifyBuffAll 缓冲队列通知所有人
	NotifyBuffAll(MsgId uint32, data []byte) error
}
