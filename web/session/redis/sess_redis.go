// Usage:
// import(
//   _ "github.com/astaxie/beego/session/redis"
//   "github.com/astaxie/beego/session"
// )
//
// 	func init() {
// 		globalSessions, _ = session.NewManager("redis", ``{"cookieName":"gosessionid","gclifetime":3600,"ProviderConfig":"127.0.0.1:7070"}``)
// 		go globalSessions.GC()
// 	}
package redis

import (
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gomodule/redigo/redis"
	"github.com/astaxie/beego/session"
)

var redispder = &Provider{}

// MaxPoolSize Redis池的大小(存储Session的个数)
var MaxPoolSize = 100

type SessionStore struct {
	p           *redis.Pool
	sid         string
	lock        sync.RWMutex
	values      map[interface{}]interface{}
	maxlifetime int64 // 生命周期
}

func (rs *SessionStore) Set(key, value interface{}) error {
	rs.lock.Lock()
	defer rs.lock.Unlock()
	rs.values[key] = value
	return nil
}

func (rs *SessionStore) Get(key interface{}) interface{} {
	rs.lock.RLock()
	defer rs.lock.RUnlock()
	if v, ok := rs.values[key]; ok {
		return v
	}
	return nil
}

func (rs *SessionStore) Delete(key interface{}) error {
	rs.lock.Lock()
	defer rs.lock.Unlock()
	delete(rs.values, key)
	return nil
}

func (rs *SessionStore) Flush() error {
	rs.lock.Lock()
	defer rs.lock.Unlock()
	rs.values = make(map[interface{}]interface{})
	return nil
}

func (rs *SessionStore) SessionID() string {
	return rs.sid
}

// 保存Session到Redis
func (rs *SessionStore) SessionRelease(w http.ResponseWriter) {
	b, err := session.EncodeGob(rs.values)
	if err != nil {
		return
	}
	c := rs.p.Get() // 获取一个Redis连接
	defer c.Close()
	c.Do("SETEX", rs.sid, rs.maxlifetime, string(b))
}

// 引擎
type Provider struct {
	maxlifetime int64
	savePath    string //Redis连接地址
	poolsize    int    //Redis池大小
	password    string //授权密码
	dbNum       int    //db号码
	poollist    *redis.Pool
}

// SessionInit init redis session
// savepath内容: "addr,poolsize,password,dbnum,IdleTimeoutSeconds"
// e.g. 127.0.0.1:6379,100,astaxie,0,30
func (rp *Provider) SessionInit(maxlifetime int64, savePath string) error {
	rp.maxlifetime = maxlifetime
	configs := strings.Split(savePath, ",")
	if len(configs) > 0 {
		rp.savePath = configs[0]
	}
	if len(configs) > 1 {
		poolsize, err := strconv.Atoi(configs[1])
		if err != nil || poolsize < 0 {
			rp.poolsize = MaxPoolSize
		} else {
			rp.poolsize = poolsize
		}
	} else {
		rp.poolsize = MaxPoolSize
	}
	if len(configs) > 2 {
		rp.password = configs[2]
	}
	if len(configs) > 3 {
		dbnum, err := strconv.Atoi(configs[3])
		if err != nil || dbnum < 0 {
			rp.dbNum = 0
		} else {
			rp.dbNum = dbnum
		}
	} else {
		rp.dbNum = 0
	}
	var idleTimeout time.Duration = 0
	if len(configs) > 4 {
		timeout, err := strconv.Atoi(configs[4])
		if err == nil && timeout > 0 {
			idleTimeout = time.Duration(timeout) * time.Second
		}
	}
	rp.poollist = &redis.Pool{
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", rp.savePath)
			if err != nil {
				return nil, err
			}
			if rp.password != "" {
				if _, err = c.Do("AUTH", rp.password); err != nil {
					c.Close()
					return nil, err
				}
			}
			// some redis proxy such as twemproxy is not support select command
			if rp.dbNum > 0 {
				_, err = c.Do("SELECT", rp.dbNum)
				if err != nil {
					c.Close()
					return nil, err
				}
			}
			return c, err
		},
		MaxIdle: rp.poolsize,
	}

	rp.poollist.IdleTimeout = idleTimeout

	return rp.poollist.Get().Err()
}

// SessionRead read redis session by sid
func (rp *Provider) SessionRead(sid string) (session.Store, error) {
	c := rp.poollist.Get()
	defer c.Close()

	var kv map[interface{}]interface{}

	kvs, err := redis.String(c.Do("GET", sid))
	if err != nil && err != redis.ErrNil {
		return nil, err
	}
	if len(kvs) == 0 {
		kv = make(map[interface{}]interface{})
	} else {
		if kv, err = session.DecodeGob([]byte(kvs)); err != nil {
			return nil, err
		}
	}

	rs := &SessionStore{p: rp.poollist, sid: sid, values: kv, maxlifetime: rp.maxlifetime}
	return rs, nil
}

// SessionExist check redis session exist by sid
func (rp *Provider) SessionExist(sid string) bool {
	c := rp.poollist.Get()
	defer c.Close()

	if existed, err := redis.Int(c.Do("EXISTS", sid)); err != nil || existed == 0 {
		return false
	}
	return true
}

// SessionRegenerate generate new sid for redis session
func (rp *Provider) SessionRegenerate(oldsid, sid string) (session.Store, error) {
	c := rp.poollist.Get()
	defer c.Close()

	if existed, _ := redis.Int(c.Do("EXISTS", oldsid)); existed == 0 {
		// oldsid doesn't exists, set the new sid directly
		// ignore error here, since if it return error
		// the existed value will be 0
		c.Do("SET", sid, "", "EX", rp.maxlifetime)
	} else {
		c.Do("RENAME", oldsid, sid)
		c.Do("EXPIRE", sid, rp.maxlifetime)
	}
	return rp.SessionRead(sid)
}

// SessionDestroy delete redis session by id
func (rp *Provider) SessionDestroy(sid string) error {
	c := rp.poollist.Get()
	defer c.Close()

	c.Do("DEL", sid)
	return nil
}

// SessionGC Impelment method, no used.
func (rp *Provider) SessionGC() {
}

// SessionAll return all activeSession
func (rp *Provider) SessionAll() int {
	return 0
}

func init() {
	session.Register("redis", redispder)
}
