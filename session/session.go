// Usage:
// import(
//   "github.com/astaxie/beego/session"
// )
//
//	func init() {
//      globalSessions, _ = session.NewManager("memory", `{"cookieName":"gosessionid", "enableSetCookie,omitempty": true, "gclifetime":3600, "maxLifetime": 3600, "secure": false, "cookieLifeTime": 3600, "providerConfig": ""}`)
//		go globalSessions.GC()
//	}
package session

import (
	"net/http"
	"os"
	"fmt"
	"net/textproto"
	"net/url"
	"time"
	"encoding/hex"
	"io"
	"errors"
	"math/rand"
	"log"
)

/**
 Session需要解决的问题:
	1. Session怎么存储值?
	2. Session的运行生命周期?
*/

// Store包含具有特定ID的一个会话进程的所有数据.
type Store interface {
	Set(key, value interface{}) error     //set session value
	Get(key interface{}) interface{}      //get session value
	Delete(key interface{}) error         //delete session value
	SessionID() string                    //back current sessionID
	SessionRelease(w http.ResponseWriter) // release the resource & save data to provider & return the data
	Flush() error                         //delete all data
}

// Provider包含全局会话方法和已保存的SessionStores, 它可以通过其id操作SessionStore.
type Provider interface {
	SessionInit(gclifetime int64, config string) error
	SessionRead(sid string) (Store, error)
	SessionExist(sid string) bool
	SessionRegenerate(oldsid, sid string) (Store, error)
	SessionDestroy(sid string) error
	SessionAll() int //get all active session
	SessionGC()
}

var provides = make(map[string]Provider)

// SLogger a helpful variable to log information about session
var SLogger = NewSessionLog(os.Stderr)

func Register(name string, provide Provider) {
	if provide == nil {
		panic("session: Register provide is nil")
	}
	if _, dup := provides[name]; dup {
		panic("session: Register called twice for provider " + name)
	}
	provides[name] = provide
}

// Session的配置信息
type ManagerConfig struct {
	CookieName              string `json:"cookieName"`
	EnableSetCookie         bool   `json:"enableSetCookie,omitempty"`
	Gclifetime              int64  `json:"gclifetime"`
	Maxlifetime             int64  `json:"maxLifetime"`
	DisableHTTPOnly         bool   `json:"disableHTTPOnly"`
	Secure                  bool   `json:"secure"`
	CookieLifeTime          int    `json:"cookieLifeTime"`
	ProviderConfig          string `json:"providerConfig"`
	Domain                  string `json:"domain"`
	SessionIDLength         int64  `json:"sessionIDLength"`
	EnableSidInHTTPHeader   bool   `json:"EnableSidInHTTPHeader"`
	SessionNameInHTTPHeader string `json:"SessionNameInHTTPHeader"`
	EnableSidInURLQuery     bool   `json:"EnableSidInURLQuery"`
}

// Manager contains Provider and its configuration.
type Manager struct {
	provider Provider
	config   *ManagerConfig
}

// 默认创建 Manager
// provideName:
// 1. cookie
// 2. file
// 3. memory
// 4. redis
// 5. mysql
// json config:
// 1. is https  default false
// 2. hashfunc  default sha1
// 3. hashkey default beegosessionkey
// 4. maxage default is none
func NewManager(provideName string, cf *ManagerConfig) (*Manager, error) {
	provider, ok := provides[provideName]
	if !ok {
		return nil, fmt.Errorf("session: unknown provide %q (forgotten import?)", provideName)
	}

	if cf.Maxlifetime == 0 {
		cf.Maxlifetime = cf.Gclifetime
	}

	if cf.EnableSidInHTTPHeader {
		// Session必须进行设置SessionHeader传给客户端, 类型是带"-"的驼峰结构, 例如"Accept-Encoding"
		if cf.SessionNameInHTTPHeader == "" {
			panic(errors.New("SessionNameInHTTPHeader is empty"))
		}

		// 返回一个MIME头的键的规范格式. 该标准会将首字母和所有"-"之后的字符改为大写, 其余字母改为小写.
		// 例如:"accept-encoding"作为键的标准格式是"Accept-Encoding". MIME头的键必须是ASCII码构成
		strMimeHeader := textproto.CanonicalMIMEHeaderKey(cf.SessionNameInHTTPHeader)
		if cf.SessionNameInHTTPHeader != strMimeHeader {
			strErrMsg := "SessionNameInHTTPHeader (" + cf.SessionNameInHTTPHeader + ") has the wrong format, it should be like this : " + strMimeHeader
			panic(errors.New(strErrMsg))
		}
	}

	err := provider.SessionInit(cf.Maxlifetime, cf.ProviderConfig) //初始化
	if err != nil {
		return nil, err
	}

	if cf.SessionIDLength == 0 {
		cf.SessionIDLength = 16
	}

	return &Manager{
		provider,
		cf,
	}, nil
}

// getSid retrieves session identifier from HTTP Request.
// First try to retrieve id by reading from cookie, session cookie name is configurable,
// if not exist, then retrieve id from querying parameters.
//
// error is not nil when there is anything wrong.
// sid is empty when need to generate a new session id
// otherwise return an valid session id.
func (manager *Manager) getSid(r *http.Request) (string, error) {
	cookie, errs := r.Cookie(manager.config.CookieName)
	if errs != nil || cookie.Value == "" {
		var sid string
		if manager.config.EnableSidInURLQuery {
			errs := r.ParseForm()
			if errs != nil {
				return "", errs
			}

			sid = r.FormValue(manager.config.CookieName)
		}

		// if not found in Cookie / param, then read it from request headers
		if manager.config.EnableSidInHTTPHeader && sid == "" {
			sids, isFound := r.Header[manager.config.SessionNameInHTTPHeader]
			if isFound && len(sids) != 0 {
				return sids[0], nil
			}
		}

		return sid, nil
	}

	// HTTP Request contains cookie for sessionid info.
	return url.QueryUnescape(cookie.Value)
}

// SessionStart generate or read the session id from http request.
// if session id exists, return SessionStore with this id.
func (manager *Manager) SessionStart(w http.ResponseWriter, r *http.Request) (session Store, err error) {
	sid, errs := manager.getSid(r)
	if errs != nil {
		return nil, errs
	}

	if sid != "" && manager.provider.SessionExist(sid) {
		return manager.provider.SessionRead(sid)
	}

	// Generate a new session
	sid, errs = manager.sessionID()
	if errs != nil {
		return nil, errs
	}

	session, err = manager.provider.SessionRead(sid)
	if err != nil {
		return nil, err
	}
	cookie := &http.Cookie{
		Name:     manager.config.CookieName,
		Value:    url.QueryEscape(sid),
		Path:     "/",
		HttpOnly: !manager.config.DisableHTTPOnly,
		Secure:   manager.isSecure(r),
		Domain:   manager.config.Domain,
	}
	if manager.config.CookieLifeTime > 0 {
		cookie.MaxAge = manager.config.CookieLifeTime
		cookie.Expires = time.Now().Add(time.Duration(manager.config.CookieLifeTime) * time.Second)
	}
	if manager.config.EnableSetCookie {
		http.SetCookie(w, cookie)
	}
	r.AddCookie(cookie)

	if manager.config.EnableSidInHTTPHeader {
		r.Header.Set(manager.config.SessionNameInHTTPHeader, sid)
		w.Header().Set(manager.config.SessionNameInHTTPHeader, sid)
	}

	return
}

// SessionDestroy Destroy session by its id in http request cookie.
func (manager *Manager) SessionDestroy(w http.ResponseWriter, r *http.Request) {
	if manager.config.EnableSidInHTTPHeader {
		r.Header.Del(manager.config.SessionNameInHTTPHeader)
		w.Header().Del(manager.config.SessionNameInHTTPHeader)
	}

	cookie, err := r.Cookie(manager.config.CookieName)
	if err != nil || cookie.Value == "" {
		return
	}

	sid, _ := url.QueryUnescape(cookie.Value)
	manager.provider.SessionDestroy(sid)
	if manager.config.EnableSetCookie {
		expiration := time.Now()
		cookie = &http.Cookie{Name: manager.config.CookieName,
			Path:     "/",
			HttpOnly: !manager.config.DisableHTTPOnly,
			Expires:  expiration,
			MaxAge:   -1}

		http.SetCookie(w, cookie)
	}
}

// GetSessionStore Get SessionStore by its id.
func (manager *Manager) GetSessionStore(sid string) (sessions Store, err error) {
	sessions, err = manager.provider.SessionRead(sid)
	return
}

// GC Start session gc process.
// it can do gc in times after gc lifetime.
func (manager *Manager) GC() {
	manager.provider.SessionGC()
	time.AfterFunc(time.Duration(manager.config.Gclifetime)*time.Second, func() { manager.GC() })
}

// SessionRegenerateID Regenerate a session id for this SessionStore who's id is saving in http request.
func (manager *Manager) SessionRegenerateID(w http.ResponseWriter, r *http.Request) (session Store) {
	sid, err := manager.sessionID()
	if err != nil {
		return
	}
	cookie, err := r.Cookie(manager.config.CookieName)
	if err != nil || cookie.Value == "" {
		//delete old cookie
		session, _ = manager.provider.SessionRead(sid)
		cookie = &http.Cookie{Name: manager.config.CookieName,
			Value:    url.QueryEscape(sid),
			Path:     "/",
			HttpOnly: !manager.config.DisableHTTPOnly,
			Secure:   manager.isSecure(r),
			Domain:   manager.config.Domain,
		}
	} else {
		oldsid, _ := url.QueryUnescape(cookie.Value)
		session, _ = manager.provider.SessionRegenerate(oldsid, sid)
		cookie.Value = url.QueryEscape(sid)
		cookie.HttpOnly = true
		cookie.Path = "/"
	}
	if manager.config.CookieLifeTime > 0 {
		cookie.MaxAge = manager.config.CookieLifeTime
		cookie.Expires = time.Now().Add(time.Duration(manager.config.CookieLifeTime) * time.Second)
	}
	if manager.config.EnableSetCookie {
		http.SetCookie(w, cookie)
	}
	r.AddCookie(cookie)

	if manager.config.EnableSidInHTTPHeader {
		r.Header.Set(manager.config.SessionNameInHTTPHeader, sid)
		w.Header().Set(manager.config.SessionNameInHTTPHeader, sid)
	}

	return
}

// GetActiveSession Get all active sessions count number.
func (manager *Manager) GetActiveSession() int {
	return manager.provider.SessionAll()
}

// SetSecure Set cookie with https.
func (manager *Manager) SetSecure(secure bool) {
	manager.config.Secure = secure
}

func (manager *Manager) sessionID() (string, error) {
	b := make([]byte, manager.config.SessionIDLength)
	n, err := rand.Read(b)
	if n != len(b) || err != nil {
		return "", fmt.Errorf("Could not successfully read from the system CSPRNG")
	}
	return hex.EncodeToString(b), nil
}

// Set cookie with https.
func (manager *Manager) isSecure(req *http.Request) bool {
	if !manager.config.Secure {
		return false
	}
	if req.URL.Scheme != "" {
		return req.URL.Scheme == "https"
	}
	if req.TLS == nil {
		return false
	}
	return true
}

// Log implement the log.Logger
type Log struct {
	*log.Logger
}

// NewSessionLog set io.Writer to create a Logger for session.
func NewSessionLog(out io.Writer) *Log {
	sl := new(Log)
	sl.Logger = log.New(out, "[SESSION]", 1e9)
	return sl
}
