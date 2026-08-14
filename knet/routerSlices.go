package knet

import (
	"fmt"

	"kinz/kiface"
)

// routerSlices implements kiface.IRouterSlices as a handle over MsgHandler.
type routerSlices struct {
	mh *MsgHandler
}

// Use implements kiface.IRouterSlices: appends global middleware.
func (r *routerSlices) Use(handlers ...kiface.RouterHandler) kiface.IRouterSlices {
	r.mh.mu.Lock()
	r.mh.globalHandlers = append(r.mh.globalHandlers, handlers...)
	r.mh.mu.Unlock()
	return r
}

// AddHandler implements kiface.IRouterSlices.
func (r *routerSlices) AddHandler(msgID uint32, handlers ...kiface.RouterHandler) error {
	return r.mh.addSlices(msgID, handlers...)
}

// Group implements kiface.IRouterSlices.
func (r *routerSlices) Group(start, end uint32, handlers ...kiface.RouterHandler) kiface.IGroupRouterSlices {
	g, err := r.mh.Group(start, end, handlers...)
	if err != nil {
		return nil
	}
	return g
}

// GetHandlers implements kiface.IRouterSlices.
func (r *routerSlices) GetHandlers(msgID uint32) ([]kiface.RouterHandler, bool) {
	r.mh.mu.RLock()
	defer r.mh.mu.RUnlock()
	h, ok := r.mh.apis[msgID]
	return h, ok
}

// groupRouterSlices implements kiface.IGroupRouterSlices: handlers scoped to a
// msgID range.
type groupRouterSlices struct {
	mh         *MsgHandler
	start, end uint32
}

// Use implements kiface.IGroupRouterSlices: appends middleware scoped to the
// group's msgID range.
func (g *groupRouterSlices) Use(handlers ...kiface.RouterHandler) kiface.IGroupRouterSlices {
	g.mh.mu.Lock()
	for i := range g.mh.groupRanges {
		if g.mh.groupRanges[i].start == g.start && g.mh.groupRanges[i].end == g.end {
			g.mh.groupRanges[i].handlers = append(g.mh.groupRanges[i].handlers, handlers...)
			break
		}
	}
	g.mh.mu.Unlock()
	return g
}

// AddHandler implements kiface.IGroupRouterSlices: registers handlers for a
// msgID inside the group range; out-of-range msgIDs return an error.
func (g *groupRouterSlices) AddHandler(msgID uint32, handlers ...kiface.RouterHandler) error {
	if msgID < g.start || msgID > g.end {
		return fmt.Errorf("kinz: msgID %d outside group range [%d, %d]", msgID, g.start, g.end)
	}
	return g.mh.addSlices(msgID, handlers...)
}
