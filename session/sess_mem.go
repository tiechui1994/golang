package session

import (
	"container/list"
	"net/http"
	"sync"
	"time"
)

var mempder = &MemProvider{list: list.New(), sessions: make(map[string]*list.Element)}

// MemSessionStore, 将数据保存到内存当中, 和SessionCookie类似
type MemSessionStore struct {
	sid          string                      //session id
	timeAccessed time.Time                   // 最后一次访问的时间
	value        map[interface{}]interface{} //session store
	lock         sync.RWMutex
}

func (st *MemSessionStore) Set(key, value interface{}) error {
	st.lock.Lock()
	defer st.lock.Unlock()
	st.value[key] = value
	return nil
}

func (st *MemSessionStore) Get(key interface{}) interface{} {
	st.lock.RLock()
	defer st.lock.RUnlock()
	if v, ok := st.value[key]; ok {
		return v
	}
	return nil
}

func (st *MemSessionStore) Delete(key interface{}) error {
	st.lock.Lock()
	defer st.lock.Unlock()
	delete(st.value, key)
	return nil
}

// 清空缓存, 直接将value重新赋值即可
func (st *MemSessionStore) Flush() error {
	st.lock.Lock()
	defer st.lock.Unlock()
	st.value = make(map[interface{}]interface{})
	return nil
}

// 获取SessionID
func (st *MemSessionStore) SessionID() string {
	return st.sid
}

// SessionRelease, 不做任何处理
func (st *MemSessionStore) SessionRelease(w http.ResponseWriter) {
}

// 内存引擎(配置只有一个, savePath)
type MemProvider struct {
	lock        sync.RWMutex             // locker
	sessions    map[string]*list.Element // 使用 map + 双向链表存储所有的Session
	list        *list.List               // for gc
	maxlifetime int64
	savePath    string
}

// 初始化
func (pder *MemProvider) SessionInit(maxlifetime int64, savePath string) error {
	pder.maxlifetime = maxlifetime
	pder.savePath = savePath
	return nil
}

// 通过SessionID 获取MemSessionStore实例 --> 存储结构
func (pder *MemProvider) SessionRead(sid string) (Store, error) {
	pder.lock.RLock()
	if element, ok := pder.sessions[sid]; ok {
		go pder.SessionUpdate(sid)
		pder.lock.RUnlock()
		return element.Value.(*MemSessionStore), nil
	}
	pder.lock.RUnlock()
	pder.lock.Lock()
	newsess := &MemSessionStore{sid: sid, timeAccessed: time.Now(), value: make(map[interface{}]interface{})}
	element := pder.list.PushFront(newsess)
	pder.sessions[sid] = element
	pder.lock.Unlock()
	return newsess, nil
}

// SessionExist check session store exist in memory session by sid
func (pder *MemProvider) SessionExist(sid string) bool {
	pder.lock.RLock()
	defer pder.lock.RUnlock()
	if _, ok := pder.sessions[sid]; ok {
		return true
	}
	return false
}

// SessionRegenerate generate new sid for session store in memory session
func (pder *MemProvider) SessionRegenerate(oldsid, sid string) (Store, error) {
	pder.lock.RLock()
	if element, ok := pder.sessions[oldsid]; ok {
		go pder.SessionUpdate(oldsid)
		pder.lock.RUnlock()
		pder.lock.Lock()
		element.Value.(*MemSessionStore).sid = sid
		pder.sessions[sid] = element
		delete(pder.sessions, oldsid)
		pder.lock.Unlock()
		return element.Value.(*MemSessionStore), nil
	}
	pder.lock.RUnlock()
	pder.lock.Lock()
	newsess := &MemSessionStore{sid: sid, timeAccessed: time.Now(), value: make(map[interface{}]interface{})}
	element := pder.list.PushFront(newsess)
	pder.sessions[sid] = element
	pder.lock.Unlock()
	return newsess, nil
}

// SessionDestroy delete session store in memory session by id
func (pder *MemProvider) SessionDestroy(sid string) error {
	pder.lock.Lock()
	defer pder.lock.Unlock()
	if element, ok := pder.sessions[sid]; ok {
		delete(pder.sessions, sid)
		pder.list.Remove(element)
		return nil
	}
	return nil
}

// SessionGC clean expired session stores in memory session
func (pder *MemProvider) SessionGC() {
	pder.lock.RLock()
	for {
		element := pder.list.Back()
		if element == nil {
			break
		}
		if (element.Value.(*MemSessionStore).timeAccessed.Unix() + pder.maxlifetime) < time.Now().Unix() {
			pder.lock.RUnlock()
			pder.lock.Lock()
			pder.list.Remove(element)
			delete(pder.sessions, element.Value.(*MemSessionStore).sid)
			pder.lock.Unlock()
			pder.lock.RLock()
		} else {
			break
		}
	}
	pder.lock.RUnlock()
}

// SessionAll get count number of memory session
func (pder *MemProvider) SessionAll() int {
	return pder.list.Len()
}

// SessionUpdate expand time of session store by id in memory session
func (pder *MemProvider) SessionUpdate(sid string) error {
	pder.lock.Lock()
	defer pder.lock.Unlock()
	if element, ok := pder.sessions[sid]; ok {
		element.Value.(*MemSessionStore).timeAccessed = time.Now()
		pder.list.MoveToFront(element)
		return nil
	}
	return nil
}

func init() {
	Register("memory", mempder)
}
