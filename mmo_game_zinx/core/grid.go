package core

import (
	"fmt"
	"sync"
)

// Grid AOI 的一个格子类型
type Grid struct {
	GID       int // 格子 ID
	MinX      int
	MaxX      int
	MinY      int
	MaxY      int
	playerIDs map[int]bool // 当前格子内玩家或物体的集合
	pIDLock   sync.RWMutex // playerIDs 保护锁
}

// NewGrid 初始化
func NewGrid(gID, minX, maxX, minY, maxY int) *Grid {
	return &Grid{
		GID:       gID,
		MinX:      minX,
		MaxX:      maxX,
		MinY:      minY,
		MaxY:      maxY,
		playerIDs: make(map[int]bool),
	}
}

// Add 给格子添加一个玩家
func (g *Grid) Add(playerID int) {
	g.pIDLock.Lock()
	defer g.pIDLock.Unlock()
	g.playerIDs[playerID] = true
}

// Remove 删除一个玩家
func (g *Grid) Remove(playerID int) {
	g.pIDLock.Lock()
	defer g.pIDLock.Unlock()
	delete(g.playerIDs, playerID)
}

// GetPlayerIDs 得到当前格子中所有的玩家 ID
func (g *Grid) GetPlayerIDs() (playerIDs []int) {
	g.pIDLock.RLock()
	defer g.pIDLock.RUnlock()

	for k, _ := range g.playerIDs {
		playerIDs = append(playerIDs, k)
	}

	return
}

// 调试使用-打印出格子的基本信息
func (g *Grid) String() string {
	return fmt.Sprintf("Grid id: %d, MinX: %d, MaxX: %d, MinY: %d, MaxY: %d, PlayerIDs: %v",
		g.GID, g.MinX, g.MaxX, g.MinY, g.MaxY, g.GetPlayerIDs())
}
