package core

import "fmt"

// 一些 AOI 的边界值
const (
	AOI_MIN_X  int = 85
	AOI_MAX_X  int = 410
	AOI_CNTS_X int = 10
	AOI_MIN_Y  int = 75
	AOI_MAX_Y  int = 400
	AOI_CNTS_Y int = 20
)

type AOIManager struct {
	// 区域的左边界坐标
	MinX int
	MaxX int
	// X 方向格子的数量
	CntsX int
	// 区域的上边界坐标
	MinY  int
	MaxY  int
	CntsY int
	// 当前区域中有哪些格子
	grids map[int]*Grid
}

// NewAOIManager 初始化
func NewAOIManager(minX int, maxX int, cntsX int, minY int, maxY int, cntsY int) *AOIManager {
	aoiMgr := &AOIManager{
		MinX:  minX,
		MaxX:  maxX,
		CntsX: cntsX,
		MinY:  minY,
		MaxY:  maxY,
		CntsY: cntsY,
		grids: make(map[int]*Grid),
	}
	// 给 AOI 初始化区域的格子进行编号和初始化
	for y := 0; y < cntsY; y++ {
		for x := 0; x < cntsX; x++ {
			// 计算格子的 ID 根据X和Y编号 GID = Y * cntX + X
			gid := y*cntsX + x

			// 初始化 gid 格子
			aoiMgr.grids[gid] = NewGrid(gid,
				aoiMgr.MinX+x*aoiMgr.gridWidth(),
				aoiMgr.MinX+(x+1)*aoiMgr.gridWidth(),
				aoiMgr.MinY+y*aoiMgr.gridHeight(),
				aoiMgr.MinY+(y+1)*aoiMgr.gridHeight(),
			)
		}
	}

	return aoiMgr
}

// 得到每个格子在 X 方向的宽度
func (m *AOIManager) gridWidth() int {
	return (m.MaxX - m.MinX) / m.CntsX
}

// 得到每个格子在 Y 方向上的高度
func (m *AOIManager) gridHeight() int {
	return (m.MaxY - m.MinY) / m.CntsY
}

// 打印调试
func (m *AOIManager) String() string {
	// 打印 AOIManager 信息
	s := fmt.Sprintf("AOIManager\n MinX:%d, MaxX:%d, CntsX:%d, MinY:%d, MaxY:%d, CntsY:%d\n Grids in AOIManager:\n",
		m.MinX, m.MaxX, m.CntsX, m.MinY, m.MaxY, m.CntsY)

	for _, grid := range m.grids {
		s += fmt.Sprintln(grid)
	}

	return s
}

// GetSurroundGridsByGid 根据格子 GID 得到周边九宫格格子集合
func (m *AOIManager) GetSurroundGridsByGid(gID int) (grids []*Grid) {
	// 判断当前 gid 是否在该 AOIManager中
	if _, ok := m.grids[gID]; !ok {
		return
	}
	// 初始化 grids 返回值切片
	grids = append(grids, m.grids[gID])
	// 需要判断 gID 的左右边是否有格子
	// 得到当前格子的 X 轴的编号
	idx := gID % m.CntsX
	if idx > 0 {
		grids = append(grids, m.grids[gID-1])
	}
	if idx < m.CntsX-1 {
		grids = append(grids, m.grids[gID+1])
	}

	// 对 grids 中格子进行遍历，判断上下是否还有格子
	gidsX := make([]int, 0, len(grids))
	for _, v := range grids {
		gidsX = append(gidsX, v.GID)
	}
	for _, v := range gidsX {
		// 得到 Y 轴编号
		idy := v / m.CntsY
		// 判断上边
		if idy > 0 {
			grids = append(grids, m.grids[v-m.CntsX])
		}
		// 判断下边
		if idy < m.CntsY-1 {
			grids = append(grids, m.grids[v+m.CntsX])
		}
	}

	return
}

// GetGidByPos 通过 X, Y 得到当前格子 GID
func (m *AOIManager) GetGidByPos(x, y float32) int {
	idx := (int(x) - m.MinX) / m.gridWidth()
	idy := (int(y) - m.MinY) / m.gridHeight()
	return idy*m.CntsX + idx
}

// GetPidsByPos 根据横纵坐标得到周边九宫格 PlayerIDs
func (m *AOIManager) GetPidsByPos(x, y float32) (playerIDs []int) {
	// 得到当前玩家的 GID
	gID := m.GetGidByPos(x, y)
	// 通过 GID 得到周边九宫格信息
	grids := m.GetSurroundGridsByGid(gID)
	// 将九宫格的信息里的全部 PlayerIDs 累加到切片
	for _, v := range grids {
		playerIDs = append(playerIDs, v.GetPlayerIDs()...)
		fmt.Printf("===>grid ID : %d, Pids : %v\n", v.GID, v.GetPlayerIDs())
	}
	return
}

// AddPidToGrid 添加一个 Player 到一个特定格子中
func (m *AOIManager) AddPidToGrid(pID, gID int) {
	m.grids[gID].Add(pID)
}

// RemovePidFromGrid 移除一个格子中的 PlayerID
func (m *AOIManager) RemovePidFromGrid(pID, gID int) {
	m.grids[gID].Remove(pID)
}

// GetPidsByGid 通过 GID 获取全部的 PlayerID
func (m *AOIManager) GetPidsByGid(gID int) (playerIDs []int) {
	return m.grids[gID].GetPlayerIDs()
}

// AddToGridByPos 通过坐标将 Player 添加到一个格子中
func (m *AOIManager) AddToGridByPos(pID int, x, y float32) {
	gID := m.GetGidByPos(x, y)
	m.grids[gID].Add(pID)
}

// RemoveFromGridByPos 通过坐标将一个Player从一个格子中删除
func (m *AOIManager) RemoveFromGridByPos(pID int, x, y float32) {
	gID := m.GetGidByPos(x, y)
	m.grids[gID].Remove(pID)
}
