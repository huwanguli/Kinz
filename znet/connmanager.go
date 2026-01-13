package znet

import (
	"errors"
	"fmt"
	"sync"
	"zinx/ziface"
)

type ConnManager struct {
	connections map[uint32]ziface.IConnection // 管理的链接集合
	connLock    sync.RWMutex                  // 保护链接集合的读写锁
}

func (connMgr *ConnManager) Add(conn ziface.IConnection) {
	// 保护共享资源
	connMgr.connLock.Lock()
	defer connMgr.connLock.Unlock()

	// 将 Conn 加入到map中
	connMgr.connections[conn.GetConnID()] = conn
	// fmt.Println("conn add to connMgr " + fmt.Sprint(connMgr.connections[conn.GetConnID()]) + "\nConnMgr Len() " + fmt.Sprint(connMgr.Len()))
}

func (connMgr *ConnManager) Remove(conn ziface.IConnection) {
	// 保护共享资源
	connMgr.connLock.Lock()
	defer connMgr.connLock.Unlock()

	// 删除 Map 中对应的链接
	delete(connMgr.connections, conn.GetConnID())
	fmt.Println("conn remove from connMgr " + fmt.Sprint(connMgr.connections[conn.GetConnID()]))
}

func (connMgr *ConnManager) Get(connID uint32) (ziface.IConnection, error) {
	// 保护共享资源（读锁）
	connMgr.connLock.RLock()
	defer connMgr.connLock.RUnlock()

	if conn, ok := connMgr.connections[connID]; ok {
		return conn, nil
	}
	return nil, errors.New("conn not exist")
}

func (connMgr *ConnManager) Len() int {
	return len(connMgr.connections)
}

// ClearConn 删除所有链接
func (connMgr *ConnManager) ClearConn() {
	// 保护共享资源
	connMgr.connLock.Lock()
	defer connMgr.connLock.Unlock()

	// 删除 conn 并停止 conn 工作
	for connID, conn := range connMgr.connections {
		// 停止
		conn.Stop()

		// 删除
		delete(connMgr.connections, connID)
	}
	fmt.Println("connMgr ClearConn Len" + fmt.Sprint(len(connMgr.connections)))
}

// NewConnManager 创建一个链接方法
func NewConnManager() *ConnManager {
	return &ConnManager{
		connections: make(map[uint32]ziface.IConnection),
	}
}
