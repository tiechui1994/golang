package utils

import (
	"sync"
)

/**
 高并发状况下的Map实现(加锁)
*/

type BeeMap struct {
	lock *sync.RWMutex
	bm   map[interface{}]interface{}
}

func NewBeeMap() *BeeMap {
	return &BeeMap{
		lock: new(sync.RWMutex),
		bm:   make(map[interface{}]interface{}),
	}
}

func (m *BeeMap) Get(k interface{}) interface{} {
	m.lock.RLock()
	defer m.lock.RUnlock()
	if val, ok := m.bm[k]; ok {
		return val
	}
	return nil
}

// 添加/更新 kv -> true, k,v已存在, 则不做任何事情 -> false
func (m *BeeMap) Set(k interface{}, v interface{}) bool {
	m.lock.Lock()
	defer m.lock.Unlock()
	if val, ok := m.bm[k]; !ok {
		m.bm[k] = v
	} else if val != v {
		m.bm[k] = v
	} else {
		return false
	}
	return true
}

func (m *BeeMap) Exist(k interface{}) bool {
	m.lock.RLock()
	defer m.lock.RUnlock()
	_, ok := m.bm[k]
	return ok
}

func (m *BeeMap) Delete(k interface{}) {
	m.lock.Lock()
	defer m.lock.Unlock()
	delete(m.bm, k)
}

// Copy
func (m *BeeMap) Items() map[interface{}]interface{} {
	m.lock.RLock()
	defer m.lock.RUnlock()
	r := make(map[interface{}]interface{})
	for k, v := range m.bm {
		r[k] = v
	}
	return r
}

// Length
func (m *BeeMap) Count() int {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return len(m.bm)
}
