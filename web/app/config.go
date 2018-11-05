package app

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"

	"github.com/astaxie/beego/config"
	"github.com/astaxie/beego/context"
	"github.com/astaxie/beego/logs"
	"github.com/astaxie/beego/session"
	"github.com/astaxie/beego/utils"
)

// Beego的配置信息
type Config struct {
	AppName             string
	RunMode             string                 // 运行模式: dev | prod
	RouterCaseSensitive bool                   // 路由是否大小写敏感
	ServerName          string                 // Header参数(Server)
	RecoverPanic        bool                   // 默认是true, 启用panic的recover机制
	RecoverFunc         func(*context.Context) // 当RecoverPanic为True的时候起效
	CopyRequestBody     bool                   // ???
	EnableGzip          bool
	MaxMemory           int64     // 内存限制
	EnableErrorsShow    bool      // EnableErrorsShow, 展示错误()
	EnableErrorsRender  bool      // EnableErrorsRender, 渲染错误()
	Listen              Listen    // 监听配置, http/https
	WebConfig           WebConfig // Web配置, Web相关, Session, Render
	Log                 LogConfig // Log配置, 日志相关
}

type Listen struct {
	Graceful          bool // Graceful means use graceful module to start the server
	ServerTimeOut     int64
	ListenTCP4        bool
	EnableHTTP        bool
	HTTPAddr          string
	HTTPPort          int
	AutoTLS           bool
	Domains           []string
	TLSCacheDir       string
	EnableHTTPS       bool
	EnableMutualHTTPS bool
	HTTPSAddr         string
	HTTPSPort         int
	HTTPSCertFile     string
	HTTPSKeyFile      string
	TrustCaFile       string
	EnableAdmin       bool
	AdminAddr         string
	AdminPort         int
	EnableFcgi        bool
	EnableStdIo       bool // EnableStdIo works with EnableFcgi Use FCGI via standard I/O
}

type WebConfig struct {
	AutoRender             bool
	EnableDocs             bool
	FlashName              string
	FlashSeparator         string
	DirectoryIndex         bool
	StaticDir              map[string]string
	StaticExtensionsToGzip []string
	TemplateLeft           string
	TemplateRight          string
	ViewsPath              string
	EnableXSRF             bool
	XSRFKey                string
	XSRFExpire             int
	Session                SessionConfig
}

type SessionConfig struct {
	SessionOn                    bool
	SessionProvider              string
	SessionName                  string
	SessionGCMaxLifetime         int64
	SessionProviderConfig        string
	SessionCookieLifeTime        int
	SessionAutoSetCookie         bool
	SessionDomain                string
	SessionDisableHTTPOnly       bool // used to allow for cross domain cookies/javascript cookies.
	SessionEnableSidInHTTPHeader bool // enable store/get the sessionId into/from http headers
	SessionNameInHTTPHeader      string
	SessionEnableSidInURLQuery   bool // enable get the sessionId from Url Query params
}

type LogConfig struct {
	AccessLogs       bool
	EnableStaticLogs bool   // log static files requests default: false
	AccessLogsFormat string // access log format: JSON_FORMAT, APACHE_FORMAT or empty string
	FileLineNum      bool
	Outputs          map[string]string // Store Adaptor : config
}

var (
	BConfig        *Config          // BConfig是app默认配置
	AppConfig      *beegoAppConfig  // AppConfig是Config的实例,存储来自文件的配置信息
	AppPath        string           // app的绝对路径(目录: 命令文件的目录)
	GlobalSessions *session.Manager // 全局Session管理

	appConfigPath     string  // app配置文件的路径(目录: 启动命令的目录)
	appConfigProvider = "ini" // app配置文件的默认解析引擎
)

func init() {
	var (
		err      error
		filename = "app.conf"
	)

	BConfig = newBConfig()
	// AppPath, 命令目录
	if AppPath, err = filepath.Abs(filepath.Dir(os.Args[0])); err != nil {
		panic(err)
	}
	workPath, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	if os.Getenv("BEEGO_RUNMODE") != "" {
		filename = os.Getenv("BEEGO_RUNMODE") + ".app.conf"
	}
	appConfigPath = filepath.Join(workPath, "conf", filename) // 默认的配置文件: ./conf/{dev|prod}.app.conf
	if !utils.FileExists(appConfigPath) {
		appConfigPath = filepath.Join(AppPath, "conf", filename) // {AppPath}/conf/{dev|prod}.app.conf; dev,prod可选
		if !utils.FileExists(appConfigPath) {
			AppConfig = &beegoAppConfig{innerConfig: config.NewFakeConfig()}
			return
		}
	}

	// 解析APP的配置文件 -> 初始化BConfig
	if err = parseConfig(appConfigPath); err != nil {
		panic(err)
	}
}

func newBConfig() *Config {
	return &Config{
		AppName:             "beego",
		RunMode:             PROD,
		RouterCaseSensitive: true,
		ServerName:          "beegoServer:" + VERSION,
		RecoverPanic:        true,
		RecoverFunc:         recoverPanic, // recover函数
		CopyRequestBody:     false,
		EnableGzip:          false,
		MaxMemory:           1 << 26, // 64MB
		EnableErrorsShow:    true,
		EnableErrorsRender:  true,
		Listen: Listen{
			Graceful:      false,
			ServerTimeOut: 0,
			ListenTCP4:    false,
			EnableHTTP:    true,
			AutoTLS:       false,
			Domains:       []string{},
			TLSCacheDir:   ".",
			HTTPAddr:      "",
			HTTPPort:      8080,
			EnableHTTPS:   false,
			HTTPSAddr:     "",
			HTTPSPort:     10443,
			HTTPSCertFile: "",
			HTTPSKeyFile:  "",
			EnableAdmin:   false,
			AdminAddr:     "",
			AdminPort:     8088,
			EnableFcgi:    false,
			EnableStdIo:   false,
		},
		WebConfig: WebConfig{
			AutoRender:             true,
			EnableDocs:             false,
			FlashName:              "BEEGO_FLASH",
			FlashSeparator:         "BEEGOFLASH",
			DirectoryIndex:         false,
			StaticDir:              map[string]string{"/static": "static"},
			StaticExtensionsToGzip: []string{".css", ".js"},
			TemplateLeft:           "{{",
			TemplateRight:          "}}",
			ViewsPath:              "views",
			EnableXSRF:             false,
			XSRFKey:                "beegoxsrf",
			XSRFExpire:             0,
			Session: SessionConfig{
				SessionOn:                    false,
				SessionProvider:              "memory",
				SessionName:                  "beegosessionID",
				SessionGCMaxLifetime:         3600,
				SessionProviderConfig:        "",
				SessionDisableHTTPOnly:       false,
				SessionCookieLifeTime:        0, // set cookie default is the browser life
				SessionAutoSetCookie:         true,
				SessionDomain:                "",
				SessionEnableSidInHTTPHeader: false, // enable store/get the sessionId into/from http headers
				SessionNameInHTTPHeader:      "Beegosessionid",
				SessionEnableSidInURLQuery:   false, // enable get the sessionId from Url Query params
			},
		},
		Log: LogConfig{
			AccessLogs:       false,
			EnableStaticLogs: false,
			AccessLogsFormat: "APACHE_FORMAT",
			FileLineNum:      true,
			Outputs:          map[string]string{"console": ""},
		},
	}
}

func recoverPanic(ctx *context.Context) {
	if err := recover(); err != nil {
		// 特殊错误
		if err == ErrAbort {
			return
		}
		if !BConfig.RecoverPanic {
			panic(err)
		}

		// web页面显示错误, 默认是true
		if BConfig.EnableErrorsShow {
			// ErrorMaps 通用型错误模板, 默认是12个, 7个4x + 5个5x
			// fmt.Sprint(err)是errCode, 例如404, 500等
			if _, ok := ErrorMaps[fmt.Sprint(err)]; ok {
				exception(fmt.Sprint(err), ctx)
				return
			}
		}

		// web页面不显示错误, 进行log记录
		var stack string
		logs.Critical("the request url is ", ctx.Input.URL())
		logs.Critical("Handler crashed with error", err)
		for i := 1; ; i++ {
			_, file, line, ok := runtime.Caller(i)
			if !ok {
				break
			}
			logs.Critical(fmt.Sprintf("%s:%d", file, line)) // 打印出错文件位置
			stack = stack + fmt.Sprintln(fmt.Sprintf("%s:%d", file, line))
		}

		// DEV 环境下 且
		if BConfig.RunMode == DEV && BConfig.EnableErrorsRender {
			showErr(err, ctx, stack)
		}

		if ctx.Output.Status != 0 {
			ctx.ResponseWriter.WriteHeader(ctx.Output.Status)
		} else {
			ctx.ResponseWriter.WriteHeader(500)
		}
	}
}

// 解析配置文件
func parseConfig(appConfigPath string) (err error) {
	AppConfig, err = newAppConfig(appConfigProvider, appConfigPath)
	if err != nil {
		return err
	}
	return assignConfig(AppConfig)
}

func assignConfig(ac config.Configer) error {
	for _, i := range []interface{}{BConfig, &BConfig.Listen, &BConfig.WebConfig, &BConfig.Log, &BConfig.WebConfig.Session} {
		assignSingleConfig(i, ac)
	}
	// set the run mode first
	if envRunMode := os.Getenv("BEEGO_RUNMODE"); envRunMode != "" {
		BConfig.RunMode = envRunMode
	} else if runMode := ac.String("RunMode"); runMode != "" {
		BConfig.RunMode = runMode
	}

	if sd := ac.String("StaticDir"); sd != "" {
		BConfig.WebConfig.StaticDir = map[string]string{}
		sds := strings.Fields(sd)
		for _, v := range sds {
			if url2fsmap := strings.SplitN(v, ":", 2); len(url2fsmap) == 2 {
				BConfig.WebConfig.StaticDir["/"+strings.Trim(url2fsmap[0], "/")] = url2fsmap[1]
			} else {
				BConfig.WebConfig.StaticDir["/"+strings.Trim(url2fsmap[0], "/")] = url2fsmap[0]
			}
		}
	}

	if sgz := ac.String("StaticExtensionsToGzip"); sgz != "" {
		extensions := strings.Split(sgz, ",")
		fileExts := []string{}
		for _, ext := range extensions {
			ext = strings.TrimSpace(ext)
			if ext == "" {
				continue
			}
			if !strings.HasPrefix(ext, ".") {
				ext = "." + ext
			}
			fileExts = append(fileExts, ext)
		}
		if len(fileExts) > 0 {
			BConfig.WebConfig.StaticExtensionsToGzip = fileExts
		}
	}

	if lo := ac.String("LogOutputs"); lo != "" {
		// if lo is not nil or empty
		// means user has set his own LogOutputs
		// clear the default setting to BConfig.Log.Outputs
		BConfig.Log.Outputs = make(map[string]string)
		los := strings.Split(lo, ";")
		for _, v := range los {
			if logType2Config := strings.SplitN(v, ",", 2); len(logType2Config) == 2 {
				BConfig.Log.Outputs[logType2Config[0]] = logType2Config[1]
			} else {
				continue
			}
		}
	}

	//init log
	logs.Reset()
	for adaptor, config := range BConfig.Log.Outputs {
		err := logs.SetLogger(adaptor, config)
		if err != nil {
			fmt.Fprintln(os.Stderr, fmt.Sprintf("%s with the config %q got err:%s", adaptor, config, err.Error()))
		}
	}
	logs.SetLogFuncCall(BConfig.Log.FileLineNum)

	return nil
}

func assignSingleConfig(p interface{}, ac config.Configer) {
	pt := reflect.TypeOf(p)
	if pt.Kind() != reflect.Ptr {
		return
	}
	pt = pt.Elem()
	if pt.Kind() != reflect.Struct {
		return
	}
	pv := reflect.ValueOf(p).Elem()

	for i := 0; i < pt.NumField(); i++ {
		pf := pv.Field(i)
		if !pf.CanSet() {
			continue
		}
		name := pt.Field(i).Name
		switch pf.Kind() {
		case reflect.String:
			pf.SetString(ac.DefaultString(name, pf.String()))
		case reflect.Int, reflect.Int64:
			pf.SetInt(ac.DefaultInt64(name, pf.Int()))
		case reflect.Bool:
			pf.SetBool(ac.DefaultBool(name, pf.Bool()))
		case reflect.Struct:
		default:
			//do nothing here
		}
	}

}

// --------------------------------------------------------------------------------------------------------

// LoadAppConfig allow developer to apply a config file
func LoadAppConfig(adapterName, configPath string) error {
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return err
	}

	if !utils.FileExists(absConfigPath) {
		return fmt.Errorf("the target config file: %s don't exist", configPath)
	}

	appConfigPath = absConfigPath
	appConfigProvider = adapterName

	return parseConfig(appConfigPath)
}

type beegoAppConfig struct {
	innerConfig config.Configer
}

func newAppConfig(appConfigProvider, appConfigPath string) (*beegoAppConfig, error) {
	ac, err := config.NewConfig(appConfigProvider, appConfigPath)
	if err != nil {
		return nil, err
	}
	return &beegoAppConfig{ac}, nil
}

func (b *beegoAppConfig) Set(key, val string) error {
	if err := b.innerConfig.Set(BConfig.RunMode+"::"+key, val); err != nil {
		return err
	}
	return b.innerConfig.Set(key, val)
}

func (b *beegoAppConfig) String(key string) string {
	if v := b.innerConfig.String(BConfig.RunMode + "::" + key); v != "" {
		return v
	}
	return b.innerConfig.String(key)
}

func (b *beegoAppConfig) Strings(key string) []string {
	if v := b.innerConfig.Strings(BConfig.RunMode + "::" + key); len(v) > 0 {
		return v
	}
	return b.innerConfig.Strings(key)
}

func (b *beegoAppConfig) Int(key string) (int, error) {
	if v, err := b.innerConfig.Int(BConfig.RunMode + "::" + key); err == nil {
		return v, nil
	}
	return b.innerConfig.Int(key)
}

func (b *beegoAppConfig) Int64(key string) (int64, error) {
	if v, err := b.innerConfig.Int64(BConfig.RunMode + "::" + key); err == nil {
		return v, nil
	}
	return b.innerConfig.Int64(key)
}

func (b *beegoAppConfig) Bool(key string) (bool, error) {
	if v, err := b.innerConfig.Bool(BConfig.RunMode + "::" + key); err == nil {
		return v, nil
	}
	return b.innerConfig.Bool(key)
}

func (b *beegoAppConfig) Float(key string) (float64, error) {
	if v, err := b.innerConfig.Float(BConfig.RunMode + "::" + key); err == nil {
		return v, nil
	}
	return b.innerConfig.Float(key)
}

func (b *beegoAppConfig) DefaultString(key string, defaultVal string) string {
	if v := b.String(key); v != "" {
		return v
	}
	return defaultVal
}

func (b *beegoAppConfig) DefaultStrings(key string, defaultVal []string) []string {
	if v := b.Strings(key); len(v) != 0 {
		return v
	}
	return defaultVal
}

func (b *beegoAppConfig) DefaultInt(key string, defaultVal int) int {
	if v, err := b.Int(key); err == nil {
		return v
	}
	return defaultVal
}

func (b *beegoAppConfig) DefaultInt64(key string, defaultVal int64) int64 {
	if v, err := b.Int64(key); err == nil {
		return v
	}
	return defaultVal
}

func (b *beegoAppConfig) DefaultBool(key string, defaultVal bool) bool {
	if v, err := b.Bool(key); err == nil {
		return v
	}
	return defaultVal
}

func (b *beegoAppConfig) DefaultFloat(key string, defaultVal float64) float64 {
	if v, err := b.Float(key); err == nil {
		return v
	}
	return defaultVal
}

func (b *beegoAppConfig) DIY(key string) (interface{}, error) {
	return b.innerConfig.DIY(key)
}

func (b *beegoAppConfig) GetSection(section string) (map[string]string, error) {
	return b.innerConfig.GetSection(section)
}

func (b *beegoAppConfig) SaveConfigFile(filename string) error {
	return b.innerConfig.SaveConfigFile(filename)
}
