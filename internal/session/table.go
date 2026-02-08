package session

import (
	"net"
	"sync"
)

type Table struct {
	mu       sync.RWMutex
	idToAddr map[uint64]*net.UDPAddr
	addrToID map[string]uint64
}

func NewTable() *Table {
	return &Table{
		idToAddr: make(map[uint64]*net.UDPAddr),
		addrToID: make(map[string]uint64),
	}
}

func (t *Table) Set(id uint64, addr *net.UDPAddr) {
	if addr == nil {
		return
	}
	key := addr.String()
	t.mu.Lock()
	defer t.mu.Unlock()
	t.idToAddr[id] = addr
	t.addrToID[key] = id
}

func (t *Table) Addr(id uint64) (*net.UDPAddr, bool) {
	t.mu.RLock()
	defer t.mu.RUnlock()
	addr, ok := t.idToAddr[id]
	return addr, ok
}

func (t *Table) SessionID(addr *net.UDPAddr) (uint64, bool) {
	if addr == nil {
		return 0, false
	}
	t.mu.RLock()
	defer t.mu.RUnlock()
	id, ok := t.addrToID[addr.String()]
	return id, ok
}
