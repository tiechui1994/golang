package app

import (
	"encoding/json"
	"mime"
	"net/http"
	"path/filepath"

	"github.com/astaxie/beego/context"
	"github.com/astaxie/beego/logs"
	"github.com/astaxie/beego/session"
)

// 注册Mime, 常用的mime
func registerMime() error {
	for k, v := range mimemaps {
		mime.AddExtensionType(k, v)
	}
	return nil
}

// 注册默认的error Handler, 401, 402, 403, 404, 405, 501, 502, 503, 504, 417, 422
func registerDefaultErrorHandler() error {
	m := map[string]func(http.ResponseWriter, *http.Request){
		"401": unauthorized,
		"402": paymentRequired,
		"403": forbidden,
		"404": notFound,
		"405": methodNotAllowed,
		"500": internalServerError,
		"501": notImplemented,
		"502": badGateway,
		"503": serviceUnavailable,
		"504": gatewayTimeout,
		"417": invalidxsrf,
		"422": missingxsrf,
	}
	for e, h := range m {
		if _, ok := ErrorMaps[e]; !ok {
			ErrorHandler(e, h) // 在ErrorMaps中添加记录
		}
	}
	return nil
}

// 注册Session, 只有开启Session才会调用
func registerSession() error {
	if BConfig.WebConfig.Session.SessionOn {
		var err error
		sessionConfig := AppConfig.String("sessionConfig")
		conf := new(session.ManagerConfig)
		if sessionConfig == "" {
			conf.CookieName = BConfig.WebConfig.Session.SessionName
			conf.EnableSetCookie = BConfig.WebConfig.Session.SessionAutoSetCookie
			conf.Gclifetime = BConfig.WebConfig.Session.SessionGCMaxLifetime
			conf.Secure = BConfig.Listen.EnableHTTPS
			conf.CookieLifeTime = BConfig.WebConfig.Session.SessionCookieLifeTime
			conf.ProviderConfig = filepath.ToSlash(BConfig.WebConfig.Session.SessionProviderConfig)
			conf.DisableHTTPOnly = BConfig.WebConfig.Session.SessionDisableHTTPOnly
			conf.Domain = BConfig.WebConfig.Session.SessionDomain
			conf.EnableSidInHTTPHeader = BConfig.WebConfig.Session.SessionEnableSidInHTTPHeader
			conf.SessionNameInHTTPHeader = BConfig.WebConfig.Session.SessionNameInHTTPHeader
			conf.EnableSidInURLQuery = BConfig.WebConfig.Session.SessionEnableSidInURLQuery
		} else {
			if err = json.Unmarshal([]byte(sessionConfig), conf); err != nil {
				return err
			}
		}
		if GlobalSessions, err = session.NewManager(BConfig.WebConfig.Session.SessionProvider, conf); err != nil {
			return err
		}
		go GlobalSessions.GC()
	}
	return nil
}

// 注册模板
func registerTemplate() error {
	defer lockViewPaths()
	if err := AddViewPath(BConfig.WebConfig.ViewsPath); err != nil {
		if BConfig.RunMode == DEV {
			logs.Warn(err)
		}
		return err
	}
	return nil
}

// 注册Admin
func registerAdmin() error {
	if BConfig.Listen.EnableAdmin {
		go beeAdminApp.Run()
	}
	return nil
}

// 注册Gzip
func registerGzip() error {
	if BConfig.EnableGzip {
		context.InitGzip(
			AppConfig.DefaultInt("gzipMinLength", -1),
			AppConfig.DefaultInt("gzipCompressLevel", -1),
			AppConfig.DefaultStrings("includedMethods", []string{"GET"}),
		)
	}
	return nil
}
