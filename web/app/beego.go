package app

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	VERSION = "1.10.1"
	DEV     = "dev"
	PROD    = "prod"
)

type M map[string]interface{}

type hookfunc func() error

var (
	hooks = make([]hookfunc, 0) // 存储hookfunc
)

// AddAPPStartHook用于注册hookfunc, hookfuncs将在beego.Run()中运行
// 比如, 启动会话, 启动中间件, 构建模板, 启动管理控制等.
func AddAPPStartHook(hf ...hookfunc) {
	hooks = append(hooks, hf...)
}

/*
 启动app
 beego.Run() default run on HttpPort
 beego.Run("localhost")
 beego.Run(":8089")
 beego.Run("127.0.0.1:8089")
*/
func Run(params ...string) {

	initBeforeHTTPRun()

	if len(params) > 0 && params[0] != "" {
		strs := strings.Split(params[0], ":")
		if len(strs) > 0 && strs[0] != "" {
			BConfig.Listen.HTTPAddr = strs[0]
		}
		if len(strs) > 1 && strs[1] != "" {
			BConfig.Listen.HTTPPort, _ = strconv.Atoi(strs[1])
		}

		BConfig.Listen.Domains = params
	}

	BeeApp.Run()
}

// 带有中间件的启动app
func RunWithMiddleWares(addr string, mws ...MiddleWare) {
	initBeforeHTTPRun()

	strs := strings.Split(addr, ":")
	if len(strs) > 0 && strs[0] != "" {
		BConfig.Listen.HTTPAddr = strs[0]
		BConfig.Listen.Domains = []string{strs[0]}
	}
	if len(strs) > 1 && strs[1] != "" {
		BConfig.Listen.HTTPPort, _ = strconv.Atoi(strs[1])
	}

	BeeApp.Run(mws...)
}

func initBeforeHTTPRun() {
	// 默认的hookfunc
	AddAPPStartHook(
		registerMime,
		registerDefaultErrorHandler,
		registerSession,
		registerTemplate,
		registerAdmin,
		registerGzip,
	)

	for _, hk := range hooks {
		if err := hk(); err != nil {
			panic(err)
		}
	}
}

// TestBeegoInit is for test package init
func TestBeegoInit(ap string) {
	path := filepath.Join(ap, "conf", "app.conf")
	os.Chdir(ap)
	InitBeegoBeforeTest(path)
}

// InitBeegoBeforeTest is for test package init
func InitBeegoBeforeTest(appConfigPath string) {
	if err := LoadAppConfig(appConfigProvider, appConfigPath); err != nil {
		panic(err)
	}
	BConfig.RunMode = "test"
	initBeforeHTTPRun()
}
