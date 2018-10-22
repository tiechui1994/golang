package session

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/json"
	"net/http"
	"net/url"
	"sync"
)

var cookiepder = &CookieProvider{}
/**
	核心内容: SessionID 等价于 SessionStore实例 (加密与解密实现)
	获取任何一个都可以得到另外一个.
*/

// Cookie存储结构: 解决存储问题
type CookieSessionStore struct {
	sid    string
	values map[interface{}]interface{} // session data
	lock   sync.RWMutex
}

func (st *CookieSessionStore) Set(key, value interface{}) error {
	st.lock.Lock()
	defer st.lock.Unlock()
	st.values[key] = value
	return nil
}

func (st *CookieSessionStore) Get(key interface{}) interface{} {
	st.lock.RLock()
	defer st.lock.RUnlock()
	if v, ok := st.values[key]; ok {
		return v
	}
	return nil
}

func (st *CookieSessionStore) Delete(key interface{}) error {
	st.lock.Lock()
	defer st.lock.Unlock()
	delete(st.values, key)
	return nil
}

// 清空存储, 直接将value重新赋值
func (st *CookieSessionStore) Flush() error {
	st.lock.Lock()
	defer st.lock.Unlock()
	st.values = make(map[interface{}]interface{})
	return nil
}

// 获取SessionID
func (st *CookieSessionStore) SessionID() string {
	return st.sid
}

// SessionRelease 将Cookie当中存储的信息写入到Response当中
func (st *CookieSessionStore) SessionRelease(w http.ResponseWriter) {
	encodedCookie, err := encodeCookie(cookiepder.block, cookiepder.config.SecurityKey, cookiepder.config.SecurityName, st.values)
	if err == nil {
		cookie := &http.Cookie{
			Name:     cookiepder.config.CookieName,
			Value:    url.QueryEscape(encodedCookie), // 与获取SessionId时的方法对应
			Path:     "/",
			HttpOnly: true,
			Secure:   cookiepder.config.Secure,
			MaxAge:   cookiepder.config.Maxage}
		http.SetCookie(w, cookie)
	}
}

type cookieConfig struct {
	SecurityKey  string `json:"securityKey"`
	BlockKey     string `json:"blockKey"`
	SecurityName string `json:"securityName"`
	CookieName   string `json:"cookieName"`
	Secure       bool   `json:"secure"`
	Maxage       int    `json:"maxage"`
}

// Cookie引擎, 解决生命周期问题
type CookieProvider struct {
	maxlifetime int64
	config      *cookieConfig
	block       cipher.Block
}

// 通过配置初始化引擎. 创建过程在注册的时候已经完成
/*
json config:
 	securityKey - hash string
 	blockKey - gob encode hash string. it's saved as aes crypto.
 	securityName - recognized name in encoded cookie string
 	cookieName - cookie name
 	maxage - cookie max life time.
*/
func (pder *CookieProvider) SessionInit(maxlifetime int64, config string) error {
	pder.config = &cookieConfig{}
	err := json.Unmarshal([]byte(config), pder.config)
	if err != nil {
		return err
	}
	// BlockKey是对称加密的秘钥,长度只能是16、24、32字节,用以选择AES-128、AES-192、AES-256.
	if pder.config.BlockKey == "" {
		pder.config.BlockKey = string(generateRandomKey(16))
	}
	if pder.config.SecurityName == "" {
		pder.config.SecurityName = string(generateRandomKey(20))
	}
	// 选择加密算法
	pder.block, err = aes.NewCipher([]byte(pder.config.BlockKey))
	if err != nil {
		return err
	}
	pder.maxlifetime = maxlifetime
	return nil
}

// SessionRead, 获取存储的Session的Store
// decode cooke string to map and put into SessionStore with sid.
func (pder *CookieProvider) SessionRead(sid string) (Store, error) {
	maps, _ := decodeCookie(pder.block,
		pder.config.SecurityKey,
		pder.config.SecurityName,
		sid, pder.maxlifetime)
	if maps == nil {
		maps = make(map[interface{}]interface{})
	}
	rs := &CookieSessionStore{sid: sid, values: maps}
	return rs, nil
}

// SessionExist Cookie session is always existed
func (pder *CookieProvider) SessionExist(sid string) bool {
	return true
}

// SessionRegenerate Implement method, no used.
func (pder *CookieProvider) SessionRegenerate(oldsid, sid string) (Store, error) {
	return nil, nil
}

// SessionDestroy Implement method, no used.
func (pder *CookieProvider) SessionDestroy(sid string) error {
	return nil
}

// SessionGC Implement method, no used.
func (pder *CookieProvider) SessionGC() {
}

// SessionAll Implement method, return 0.
func (pder *CookieProvider) SessionAll() int {
	return 0
}

// SessionUpdate Implement method, no used.
func (pder *CookieProvider) SessionUpdate(sid string) error {
	return nil
}

func init() {
	Register("cookie", cookiepder)
}
