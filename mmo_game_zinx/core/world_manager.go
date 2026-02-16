package core

import (
	"sync"
)

/*
	当前游戏的世界总管理模块
*/

type WorldManager struct {
	// AOIManager AOI 管理模块
	AoiMgr  *AOIManager
	Players map[int32]*Player
	// 保护 Players 的锁
	pLock sync.RWMutex
}

// WorldMgrObj 提供一个对外的世界管理模块（全局）
var WorldMgrObj *WorldManager

// 初始化
func init() {
	WorldMgrObj = &WorldManager{
		// 创建世界 AOI 地图
		AoiMgr:  NewAOIManager(AOI_MIN_X, AOI_MAX_X, AOI_CNTS_X, AOI_MIN_Y, AOI_MAX_Y, AOI_CNTS_Y),
		Players: make(map[int32]*Player),
	}
}

// AddPlayer 添加一个玩家
func (wm *WorldManager) AddPlayer(player *Player) {
	wm.pLock.Lock()
	defer wm.pLock.Unlock()
	wm.Players[player.Pid] = player

	// 将 player 添加到 AOIMgr中
	wm.AoiMgr.AddToGridByPos(int(player.Pid), player.X, player.Z)
}

// RemovePlayerByPid 删除一个玩家
func (wm *WorldManager) RemovePlayerByPid(pid int32) {
	player := wm.Players[pid]
	wm.AoiMgr.RemoveFromGridByPos(int(player.Pid), player.X, player.Z)

	wm.pLock.Lock()
	defer wm.pLock.Unlock()
	delete(wm.Players, pid)

}

// GetPlayerByPid 通过玩家 ID 查询 Player 对象
func (wm *WorldManager) GetPlayerByPid(pid int32) *Player {
	wm.pLock.RLock()
	defer wm.pLock.RUnlock()
	return wm.Players[pid]
}

// GetAllPlayers 获取全部的在线玩家
func (wm *WorldManager) GetAllPlayers() []*Player {
	players := make([]*Player, 0)
	wm.pLock.RLock()
	defer wm.pLock.RUnlock()
	for _, player := range wm.Players {
		players = append(players, player)
	}
	return players
}
