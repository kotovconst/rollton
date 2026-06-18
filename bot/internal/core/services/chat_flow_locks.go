package services

import (
	"sync"

	"github.com/google/uuid"
)

// ChatLockMap holds one mutex per chat id, allocated on demand. Calls for the
// same id serialize; calls for different ids run in parallel.
//
// Scope: in-process only. Multi-instance deploys would need a distributed
// lock; that is out of scope.
type ChatLockMap struct {
	mu    sync.Mutex
	locks map[uuid.UUID]*sync.Mutex
}

func NewChatLockMap() *ChatLockMap {
	return &ChatLockMap{locks: make(map[uuid.UUID]*sync.Mutex)}
}

func (m *ChatLockMap) Lock(id uuid.UUID) {
	m.mu.Lock()
	l, ok := m.locks[id]
	if !ok {
		l = &sync.Mutex{}
		m.locks[id] = l
	}
	m.mu.Unlock()
	l.Lock()
}

func (m *ChatLockMap) Unlock(id uuid.UUID) {
	m.mu.Lock()
	l, ok := m.locks[id]
	m.mu.Unlock()
	if !ok {
		panic("chat_flow_locks: Unlock of unknown id (caller did not Lock first)")
	}
	l.Unlock()
}
