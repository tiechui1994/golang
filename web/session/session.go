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
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"time"
)

/**
 Session需要解决的问题:
	1. Session 怎么存储值?
	2. Session 生命周期控制: 初始化, 读取内容, 销毁, 重新生成, 垃圾回收
*/

// Session 存储
type Store interface {
	Set(key, value interface{}) error     // set session value
	Get(key interface{}) interface{}      // get session value
	Delete(key interface{}) error         // delete session value
	SessionID() string                    // get sessionID
	SessionRelease(w http.ResponseWriter) // release the resource & save data to provider & return the data
	Flush() error                         // delete all data
}

// Session引擎, Session生命周期控制
type Provider interface {
	SessionInit(gclifetime int64, config string) error
	SessionRead(sid string) (Store, error)
	SessionExist(sid string) bool
	SessionRegenerate(oldsid, sid string) (Store, error)
	SessionDestroy(sid string) error
	SessionAll() int // get all active session
	SessionGC()
}

// 注册的Session引擎的实例, 一个Session引擎, 全局只有唯一的一个实例, 这个实例是在初始化之前已经创建
var provides = make(map[string]Provider)

// Session的log
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

//--------------------------------------------------------------------------------------------------------

// Session 引擎配置
type ManagerConfig struct {
	Gclifetime      int64 `json:"gclifetime"`      // GC时间
	Maxlifetime     int64 `json:"maxLifetime"`     // 生命周期
	SessionIDLength int64 `json:"sessionIDLength"` // SessionID 长度

	// Cookie方式存储 Session
	EnableSetCookie bool   `json:"enableSetCookie,omitempty"`
	CookieName      string `json:"cookieName"` // 既可以作为Cookie当中的name, 也可以作为URL当中的name
	CookieLifeTime  int    `json:"cookieLifeTime"`

	// SessionID 的传递
	EnableSidInHTTPHeader   bool   `json:"EnableSidInHTTPHeader"`   // 是否允许 Header 当中传递SessionID
	SessionNameInHTTPHeader string `json:"SessionNameInHTTPHeader"` // EnableSidInHTTPHeader为true的时候,Header的名称
	EnableSidInURLQuery     bool   `json:"EnableSidInURLQuery"`     // 是否允许 URL 当中传递SessionID

	// 安全, 域名相关配置
	DisableHTTPOnly bool   `json:"disableHTTPOnly"`
	Secure          bool   `json:"secure"`
	ProviderConfig  string `json:"providerConfig"` // 引擎配置
	Domain          string `json:"domain"`
}

type Manager struct {
	provider Provider // Session引擎
	config   *ManagerConfig
}

/*
 构建一个特定的Session引擎的Manager, (Session引擎是单独管理的, 和log不大一样), 主要进行Session引擎的初始化工作
 provideName:
	 1. cookie
	 2. file
	 3. memory
	 4. redis
	 5. mysql
 json config:
	 1. ishttps  default false
	 2. hashfunc  default sha1
	 3. hashkey default beegosessionkey
	 4. maxage default is none
*/
func NewManager(provideName string, cf *ManagerConfig) (*Manager, error) {
	provider, ok := provides[provideName]
	if !ok {
		return nil, fmt.Errorf("session: unknown provide %q (forgotten import?)", provideName)
	}

	if cf.Maxlifetime == 0 {
		cf.Maxlifetime = cf.Gclifetime
	}

	// Header 当中传递SessionID
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

	// 初始化, 很重要
	err := provider.SessionInit(cf.Maxlifetime, cf.ProviderConfig)
	if err != nil {
		return nil, err
	}

	// 默认的配置(SessionID的长度)
	if cf.SessionIDLength == 0 {
		cf.SessionIDLength = 16
	}

	return &Manager{
		provider,
		cf,
	}, nil
}

// 针对某个请求启动Session, 先从Request当中读取SessionID, 读取失败或者不存在, 则生成一个SessionId
func (manager *Manager) SessionStart(w http.ResponseWriter, r *http.Request) (session Store, err error) {
	sid, errs := manager.getSid(r)
	if errs != nil {
		return nil, errs
	}

	// 已经有SessionID, 说明Session已经建立, 可能存储了内容
	if sid != "" && manager.provider.SessionExist(sid) {
		return manager.provider.SessionRead(sid)
	}

	// 新生成SessionID, 这里说明Session当中没有存储任何内容
	sid, errs = manager.sessionID()
	if errs != nil {
		return nil, errs
	}

	// 构建一个 Session Store
	_, err = manager.provider.SessionRead(sid)
	if err != nil {
		return nil, err
	}

	// request当中添加 cookie 内容
	/*
		Cookie 参数说明:
		Name : cookie的名称
		Value : cookie名称对应的值
		Domain : cookie的作用域, 注意这个值的要求
		Expires : 设置cookie的过期时间

		HttpOnly : 设置httpOnly属性(说明:Cookie的HttpOnly属性,指示浏览器不要在除HTTP(和HTTPS)请求之外暴露Cookie,
			一个有HttpOnly属性的Cookie,不能通过非HTTP方式来访问, 例如通过调用JavaScript(例如,引用document.cookie),
			因此,不可能通过跨域脚本来偷走这种Cookie.

		Secure : 设置Secure属性(说明: Cookie的Secure属性, 意味着保持Cookie通信只限于加密传输,指示浏览器仅仅在通过安全
			加密连接才能使用该Cookie, 如果一个Web服务器从一个非安全连接里设置了一个带有secure属性的Cookie, 当Cookie被
			发送到客户端时,它仍然能通过中间人攻击来拦截)

		MaxAge : 设置过期时间,对应浏览器cookie的MaxAge属性
	*/
	cookie := &http.Cookie{
		Name:     manager.config.CookieName,
		Value:    url.QueryEscape(sid),
		Path:     "/",
		HttpOnly: !manager.config.DisableHTTPOnly,
		Secure:   manager.isSecure(r),
		Domain:   manager.config.Domain,
	}

	// cookie的生命周期
	if manager.config.CookieLifeTime > 0 {
		cookie.MaxAge = manager.config.CookieLifeTime
		cookie.Expires = time.Now().Add(time.Duration(manager.config.CookieLifeTime) * time.Second)
	}
	// response 当中添加 cookie
	if manager.config.EnableSetCookie {
		http.SetCookie(w, cookie)
	}
	r.AddCookie(cookie)

	if manager.config.EnableSidInHTTPHeader { // 允许在Header当中设置Cookie
		r.Header.Set(manager.config.SessionNameInHTTPHeader, sid)
		w.Header().Set(manager.config.SessionNameInHTTPHeader, sid)
	}

	return
}

// 获取SessionID
func (manager *Manager) getSid(r *http.Request) (string, error) {
	// 从 Cookie 当中获取
	cookie, errs := r.Cookie(manager.config.CookieName) // CookieName (1)
	if errs != nil || cookie.Value == "" {
		var sid string

		// 从 URL 当中获取
		if manager.config.EnableSidInURLQuery {
			errs := r.ParseForm()
			if errs != nil {
				return "", errs
			}

			sid = r.FormValue(manager.config.CookieName) // CookieName (2)
		}

		// 从 Header 当中获取
		if manager.config.EnableSidInHTTPHeader && sid == "" {
			sids, isFound := r.Header[manager.config.SessionNameInHTTPHeader]
			if isFound && len(sids) != 0 {
				return sids[0], nil
			}
		}

		return sid, nil
	}

	// url.QueryEscape(s string) string  对s进行转码使之可以安全的用在URL查询里
	// url.QueryUnescape(s string) (string, error) 用于将QueryEscape转码的字符串还原.
	// 它会把%AB改为字节0xAB, 将'+'改为' '.
	// 在cookie存储的时候会调用 QueryEscape()方法
	return url.QueryUnescape(cookie.Value)
}

// 生成SessionID, 使用的是"crypto/rand"包的Read方法 (随机值)
func (manager *Manager) sessionID() (string, error) {
	b := make([]byte, manager.config.SessionIDLength)
	n, err := rand.Read(b)
	if n != len(b) || err != nil {
		return "", fmt.Errorf("Could not successfully read from the system CSPRNG")
	}
	return hex.EncodeToString(b), nil
}

// 从某次请求当中销毁Session
func (manager *Manager) SessionDestroy(w http.ResponseWriter, r *http.Request) {
	if manager.config.EnableSidInHTTPHeader { // 清除Header当中的Cookie
		r.Header.Del(manager.config.SessionNameInHTTPHeader)
		w.Header().Del(manager.config.SessionNameInHTTPHeader)
	}

	cookie, err := r.Cookie(manager.config.CookieName)
	if err != nil || cookie.Value == "" {
		return
	}

	sid, _ := url.QueryUnescape(cookie.Value)
	manager.provider.SessionDestroy(sid) // 删除Session的存储数据
	if manager.config.EnableSetCookie {  // 删除Response的Cookie(删除客户端的Cookie)
		expiration := time.Now()
		cookie = &http.Cookie{
			Name:     manager.config.CookieName,
			Path:     "/",
			HttpOnly: !manager.config.DisableHTTPOnly,
			Expires:  expiration, // 到期
			MaxAge:   -1,         // 时间为负值
		}

		http.SetCookie(w, cookie)
	}
}

// GetSessionStore 获取Session的存储结构
func (manager *Manager) GetSessionStore(sid string) (sessions Store, err error) {
	sessions, err = manager.provider.SessionRead(sid)
	return
}

// GC启动会话gc进程.
// 它可以在gc生命周期后的时间内执行gc.
func (manager *Manager) GC() {
	manager.provider.SessionGC()
	// 定时器, 每隔一定时间自调用一次
	time.AfterFunc(time.Duration(manager.config.Gclifetime)*time.Second, func() { manager.GC() })
}

// SessionRegenerateID 重新生成此会话ID的会话ID, 其id正在http请求中保存.
func (manager *Manager) SessionRegenerateID(w http.ResponseWriter, r *http.Request) (session Store) {
	sid, err := manager.sessionID()
	if err != nil {
		return
	}
	// func (r *Request) Cookie(name string) (*Cookie, error) 返回请求中名为name的Cookie,
	// 如果未找到该Cookie会返回nil, ErrNoCookie
	cookie, err := r.Cookie(manager.config.CookieName)
	if err != nil || cookie.Value == "" { // 请求当中没有cookie, 创建一个cookie
		//delete old cookie
		session, _ = manager.provider.SessionRead(sid) //从Session当中获取Session的Store
		cookie = &http.Cookie{Name: manager.config.CookieName,
			Value:    url.QueryEscape(sid),
			Path:     "/",
			HttpOnly: !manager.config.DisableHTTPOnly,
			Secure:   manager.isSecure(r),
			Domain:   manager.config.Domain,
		}
	} else {
		// 找到Cookie
		oldsid, _ := url.QueryUnescape(cookie.Value)                 // 通过value查找SessionID
		session, _ = manager.provider.SessionRegenerate(oldsid, sid) // 生成新的session

		// 更新cookie的内容
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

// GetActiveSession 活跃的Session数量.
func (manager *Manager) GetActiveSession() int {
	return manager.provider.SessionAll()
}

// SetSecure Set cookie with https.
func (manager *Manager) SetSecure(secure bool) {
	manager.config.Secure = secure
}

// Set cookie with https.
func (manager *Manager) isSecure(req *http.Request) bool {
	if !manager.config.Secure { // 先看配置Secure为 "", 0, false
		return false
	}
	if req.URL.Scheme != "" { // 看请求的Schema
		return req.URL.Scheme == "https"
	}
	if req.TLS == nil { // 看请求TLS设置
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
