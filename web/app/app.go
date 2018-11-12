package app

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"path"
	"strings"
	"time"

	"github.com/astaxie/beego/grace"
	"github.com/astaxie/beego/logs"
	"github.com/astaxie/beego/utils"
	"golang.org/x/crypto/acme/autocert"
)

var (
	BeeApp *App // app实例
)

func init() {
	BeeApp = NewApp()
}

// App使用新的PatternServeMux定义了beego应用程序
type App struct {
	Handlers *ControllerRegister // 实现了ServeHTTP方法, 即Handler实例, 除此之外, 还保存了其他信息
	Server   *http.Server
}

func NewApp() *App {
	cr := NewControllerRegister()
	app := &App{Handlers: cr, Server: &http.Server{}}
	return app
}

// 中间件: MiddleWare是对Handler的过滤处理, 其返回结果是一个Handler
type MiddleWare func(http.Handler) http.Handler

// 启动模式:
// 1. FastCGI
// 2. HTTP
// 3. HTTPS
func (app *App) Run(mws ...MiddleWare) {
	addr := BConfig.Listen.HTTPAddr

	if BConfig.Listen.HTTPPort != 0 {
		addr = fmt.Sprintf("%s:%d", BConfig.Listen.HTTPAddr, BConfig.Listen.HTTPPort)
	}

	var (
		err        error
		l          net.Listener
		endRunning = make(chan bool, 1)
	)

	// run cgi server
	if BConfig.Listen.EnableFcgi {
		if BConfig.Listen.EnableStdIo { // 通过fastcgi标准I/O, 启用fastcgi后才生效, 类似Unix进程间通信
			if err = fcgi.Serve(nil, app.Handlers); err == nil {
				logs.Info("Use FCGI via standard I/O")
			} else {
				logs.Critical("Cannot use FCGI via standard I/O", err)
			}
			return
		}

		if BConfig.Listen.HTTPPort == 0 { // 本地监听, 采用的是unix进程间通信
			if utils.FileExists(addr) {
				os.Remove(addr)
			}
			l, err = net.Listen("unix", addr)
		} else { // 远程监听, 采用tcp进程通信
			l, err = net.Listen("tcp", addr)
		}
		if err != nil {
			logs.Critical("Listen: ", err)
		}
		if err = fcgi.Serve(l, app.Handlers); err != nil {
			logs.Critical("fcgi.Serve: ", err)
		}

		return
	}

	// set handler
	app.Server.Handler = app.Handlers
	for i := len(mws) - 1; i >= 0; i-- {
		if mws[i] == nil {
			continue
		}
		// MiddleWare是对Handler的过滤处理, 其返回结果是一个Handler, 这里的操作是改造当前的Handler
		app.Server.Handler = mws[i](app.Server.Handler)
	}
	app.Server.ReadTimeout = time.Duration(BConfig.Listen.ServerTimeOut) * time.Second
	app.Server.WriteTimeout = time.Duration(BConfig.Listen.ServerTimeOut) * time.Second
	app.Server.ErrorLog = logs.GetLogger("HTTP")

	// run graceful mode(优雅启动)
	if BConfig.Listen.Graceful {
		httpsAddr := BConfig.Listen.HTTPSAddr
		app.Server.Addr = httpsAddr
		// 开启 HTTPS 安全模式
		if BConfig.Listen.EnableHTTPS || BConfig.Listen.EnableMutualHTTPS {
			go func() {
				time.Sleep(1000 * time.Microsecond)
				// addr
				if BConfig.Listen.HTTPSPort != 0 {
					httpsAddr = fmt.Sprintf("%s:%d", BConfig.Listen.HTTPSAddr, BConfig.Listen.HTTPSPort)
					app.Server.Addr = httpsAddr
				}

				// server, 这里对原始server进行了特殊处理.
				server := grace.NewServer(httpsAddr, app.Handlers) // httpsAddr进行了处理
				server.Server.ReadTimeout = app.Server.ReadTimeout
				server.Server.WriteTimeout = app.Server.WriteTimeout

				// EnableHTTPS: 开启HTTPS认证, 基本的HTTPS认证 -> AutoTLS: 自动TSL, ListenAndServeTLS()
				// EnableMutualHTTPS: 双方都进行HTTPS认证, 客户端认证服务器, 服务器认证客户端, ListenAndServeMutualTLS()
				if BConfig.Listen.EnableMutualHTTPS {
					if err := server.ListenAndServeMutualTLS(BConfig.Listen.HTTPSCertFile, BConfig.Listen.HTTPSKeyFile,
						BConfig.Listen.TrustCaFile); err != nil {
						logs.Critical("ListenAndServeTLS: ", err, fmt.Sprintf("%d", os.Getpid()))
						time.Sleep(100 * time.Microsecond)
						endRunning <- true
					}
				} else {
					// Server端的HTTPS
					if BConfig.Listen.AutoTLS {
						m := autocert.Manager{
							Prompt:     autocert.AcceptTOS,
							HostPolicy: autocert.HostWhitelist(BConfig.Listen.Domains...),
							Cache:      autocert.DirCache(BConfig.Listen.TLSCacheDir),
						}
						app.Server.TLSConfig = &tls.Config{GetCertificate: m.GetCertificate}
						BConfig.Listen.HTTPSCertFile, BConfig.Listen.HTTPSKeyFile = "", ""
					}

					if err := server.ListenAndServeTLS(BConfig.Listen.HTTPSCertFile, BConfig.Listen.HTTPSKeyFile); err != nil {
						logs.Critical("ListenAndServeTLS: ", err, fmt.Sprintf("%d", os.Getpid()))
						time.Sleep(100 * time.Microsecond)
						endRunning <- true
					}
				}
			}()
		}

		// 开启 HTTP 模式
		if BConfig.Listen.EnableHTTP {
			go func() {
				server := grace.NewServer(addr, app.Handlers)
				server.Server.ReadTimeout = app.Server.ReadTimeout
				server.Server.WriteTimeout = app.Server.WriteTimeout
				if BConfig.Listen.ListenTCP4 {
					server.Network = "tcp4"
				}
				if err := server.ListenAndServe(); err != nil {
					logs.Critical("ListenAndServe: ", err, fmt.Sprintf("%d", os.Getpid()))
					time.Sleep(100 * time.Microsecond)
					endRunning <- true
				}
			}()
		}
		<-endRunning
		return
	}

	// run normal mode
	if BConfig.Listen.EnableHTTPS || BConfig.Listen.EnableMutualHTTPS {
		go func() {
			// EnableHTTPS: 开启HTTPS认证, 基本的HTTPS认证 -> AutoTLS: 自动TSL
			// EnableMutualHTTPS: 双方都进行HTTPS认证, 客户端认证服务器, 服务器认证客户端

			time.Sleep(1000 * time.Microsecond)
			if BConfig.Listen.HTTPSPort != 0 {
				app.Server.Addr = fmt.Sprintf("%s:%d", BConfig.Listen.HTTPSAddr, BConfig.Listen.HTTPSPort)
			} else if BConfig.Listen.EnableHTTP {
				BeeLogger.Info("Start https server error, conflict with http. Please reset https port")
				return
			}

			logs.Info("https server Running on https://%s", app.Server.Addr)

			if BConfig.Listen.AutoTLS {
				m := autocert.Manager{
					Prompt:     autocert.AcceptTOS,
					HostPolicy: autocert.HostWhitelist(BConfig.Listen.Domains...),
					Cache:      autocert.DirCache(BConfig.Listen.TLSCacheDir),
				}
				app.Server.TLSConfig = &tls.Config{GetCertificate: m.GetCertificate}
				BConfig.Listen.HTTPSCertFile, BConfig.Listen.HTTPSKeyFile = "", ""
			} else if BConfig.Listen.EnableMutualHTTPS {
				// 双方都进行HTTPS认证的核心代码, 参考 grace/server
				pool := x509.NewCertPool()
				data, err := ioutil.ReadFile(BConfig.Listen.TrustCaFile)
				if err != nil {
					BeeLogger.Info("MutualHTTPS should provide TrustCaFile")
					return
				}
				pool.AppendCertsFromPEM(data)
				app.Server.TLSConfig = &tls.Config{
					ClientCAs:  pool,
					ClientAuth: tls.RequireAndVerifyClientCert,
				}
			}

			if err := app.Server.ListenAndServeTLS(BConfig.Listen.HTTPSCertFile, BConfig.Listen.HTTPSKeyFile); err != nil {
				logs.Critical("ListenAndServeTLS: ", err)
				time.Sleep(100 * time.Microsecond)
				endRunning <- true
			}
		}()

	}

	if BConfig.Listen.EnableHTTP {
		go func() {
			app.Server.Addr = addr
			logs.Info("http server Running on http://%s", app.Server.Addr)

			if BConfig.Listen.ListenTCP4 { // tcp
				ln, err := net.Listen("tcp4", app.Server.Addr)
				if err != nil {
					logs.Critical("ListenAndServe: ", err)
					time.Sleep(100 * time.Microsecond)
					endRunning <- true
					return
				}

				if err = app.Server.Serve(ln); err != nil {
					logs.Critical("ListenAndServe: ", err)
					time.Sleep(100 * time.Microsecond)
					endRunning <- true
					return
				}
			} else {
				if err := app.Server.ListenAndServe(); err != nil { // http
					logs.Critical("ListenAndServe: ", err)
					time.Sleep(100 * time.Microsecond)
					endRunning <- true
				}
			}
		}()
	}

	<-endRunning
}

/*
添加路由
  simple router
  beego.Router("/admin", &admin.UserController{})
  beego.Router("/admin/index", &admin.ArticleController{})

  regex router
  beego.Router("/api/:id([0-9]+)", &controllers.RController{})

  custom rules
  beego.Router("/api/list",&RestController{},"*:ListFood")
  beego.Router("/api/create",&RestController{},"post:CreateFood")
*/
func Router(rootpath string, c ControllerInterface, mappingMethods ...string) *App {
	BeeApp.Handlers.Add(rootpath, c, mappingMethods...)
	return BeeApp
}

/*
 UnregisterFixedRoute: 取消注册指定fixedRoute路由. 对于所有HTTP方法,方法参数可以是空的或"*",或者用于选择性移
 除的特定方法类型(例如"GET"或"POST")。

 Usage (replace "GET" with "*" for all methods):
  beego.UnregisterFixedRoute("/yourpreviouspath", "GET")
  beego.Router("/yourpreviouspath", yourControllerAddress, "get:GetNewPage")
*/
func UnregisterFixedRoute(fixedRoute string, method string) *App {
	subPaths := splitPath(fixedRoute)
	if method == "" || method == "*" {
		for m := range HTTPMETHOD {
			if _, ok := BeeApp.Handlers.routers[m]; !ok {
				continue
			}
			if BeeApp.Handlers.routers[m].prefix == strings.Trim(fixedRoute, "/ ") {
				findAndRemoveSingleTree(BeeApp.Handlers.routers[m])
				continue
			}
			findAndRemoveTree(subPaths, BeeApp.Handlers.routers[m], m)
		}
		return BeeApp
	}

	// Single HTTP method
	um := strings.ToUpper(method)
	if _, ok := BeeApp.Handlers.routers[um]; ok {
		if BeeApp.Handlers.routers[um].prefix == strings.Trim(fixedRoute, "/ ") {
			findAndRemoveSingleTree(BeeApp.Handlers.routers[um])
			return BeeApp
		}
		findAndRemoveTree(subPaths, BeeApp.Handlers.routers[um], um)
	}
	return BeeApp
}

func findAndRemoveTree(paths []string, entryPointTree *Tree, method string) {
	for i := range entryPointTree.fixrouters {
		if entryPointTree.fixrouters[i].prefix == paths[0] { // 前缀匹配
			if len(paths) == 1 {
				if len(entryPointTree.fixrouters[i].fixrouters) > 0 {
					// 如果路由有叶子,只删除第一个叶子, 其他的叶子保留
					if len(entryPointTree.fixrouters[i].leaves) > 0 {
						entryPointTree.fixrouters[i].leaves[0] = nil
						entryPointTree.fixrouters[i].leaves = entryPointTree.fixrouters[i].leaves[1:]
					}
				} else {
					// 路由没有叶子, 删除当前的Tree
					entryPointTree.fixrouters[i] = nil

					if i == len(entryPointTree.fixrouters)-1 {
						entryPointTree.fixrouters = entryPointTree.fixrouters[:i]
					} else {
						entryPointTree.fixrouters = append(entryPointTree.fixrouters[:i], entryPointTree.fixrouters[i+1:len(entryPointTree.fixrouters)]...)
					}
				}
				return
			}

			findAndRemoveTree(paths[1:], entryPointTree.fixrouters[i], method) // 后续匹配
		}
	}
}

func findAndRemoveSingleTree(entryPointTree *Tree) {
	if entryPointTree == nil {
		return
	}

	if len(entryPointTree.fixrouters) > 0 {
		// 如果路由有子树,只删除第一个叶子, 其他的叶子保留
		if len(entryPointTree.leaves) > 0 {
			entryPointTree.leaves[0] = nil
			entryPointTree.leaves = entryPointTree.leaves[1:]
		}
	}
}

/*
 Include: 生成Controller的注释路由文件 router/xxx.go

 usage:
 	beego.Include(&BankAccount{}, &OrderController{},&RefundController{},&ReceiptController{})
 	type BankAccount struct{
    	beego.Controller
 	}

 	register the function, ???
 	func (b *BankAccount) Mapping(){
  		b.Mapping("ShowAccount" , b.ShowAccount)
  		b.Mapping("ModifyAccount", b.ModifyAccount)
	}

 	// @router /account/:id  [get]
 	func (b *BankAccount) ShowAccount(){
    	// ...
 	}

    //@router /account/:id  [post]
	func (b *BankAccount) ModifyAccount(){
		//...
    }

	注释格式: // @router /url/path/pattern [get post head put delete options *]
*/
func Include(cList ...ControllerInterface) *App {
	BeeApp.Handlers.Include(cList...)
	return BeeApp
}

/*
  添加restful风格的Controller.
  Controller需要实现beego.ControllerInterface 并且定义 "pattern/:objectId" 访问每个资源
*/
func RESTRouter(rootpath string, c ControllerInterface) *App {
	Router(rootpath, c)
	Router(path.Join(rootpath, ":objectId"), c)
	return BeeApp
}

// 添加定义的Controller, 和App.AutoRouter相同
// beego.AddAuto(&MainContorlller{}) MainController方法, List(), Page()
// /main/list -> List()
// /main/page -> Page()
func AutoRouter(c ControllerInterface) *App {
	BeeApp.Handlers.AddAuto(c)
	return BeeApp
}

// AutoPrefix adds controller handler to BeeApp with prefix.
// it's same to App.AutoRouterWithPrefix.
// if beego.AutoPrefix("/admin",&MainContorlller{}) and MainController has methods List and Page,
// visit the url /admin/main/list to exec List function or /admin/main/page to exec Page function.
func AutoPrefix(prefix string, c ControllerInterface) *App {
	BeeApp.Handlers.AddAutoPrefix(prefix, c)
	return BeeApp
}

// Get used to register router for Get method
// usage:
//    beego.Get("/", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func Get(rootpath string, f FilterFunc) *App {
	BeeApp.Handlers.Get(rootpath, f)
	return BeeApp
}

// Post used to register router for Post method
// usage:
//    beego.Post("/api", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func Post(rootpath string, f FilterFunc) *App {
	BeeApp.Handlers.Post(rootpath, f)
	return BeeApp
}

// Delete used to register router for Delete method
// usage:
//    beego.Delete("/api", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func Delete(rootpath string, f FilterFunc) *App {
	BeeApp.Handlers.Delete(rootpath, f)
	return BeeApp
}

// Put used to register router for Put method
// usage:
//    beego.Put("/api", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func Put(rootpath string, f FilterFunc) *App {
	BeeApp.Handlers.Put(rootpath, f)
	return BeeApp
}

// Head used to register router for Head method
// usage:
//    beego.Head("/api", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func Head(rootpath string, f FilterFunc) *App {
	BeeApp.Handlers.Head(rootpath, f)
	return BeeApp
}

// Options used to register router for Options method
// usage:
//    beego.Options("/api", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func Options(rootpath string, f FilterFunc) *App {
	BeeApp.Handlers.Options(rootpath, f)
	return BeeApp
}

// Patch used to register router for Patch method
// usage:
//    beego.Patch("/api", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func Patch(rootpath string, f FilterFunc) *App {
	BeeApp.Handlers.Patch(rootpath, f)
	return BeeApp
}

// Any used to register router for all methods
// usage:
//    beego.Any("/api", func(ctx *context.Context){
//          ctx.Output.Body("hello world")
//    })
func Any(rootpath string, f FilterFunc) *App {
	BeeApp.Handlers.Any(rootpath, f)
	return BeeApp
}

// Handler used to register a Handler router
// usage:
//    beego.Handler("/api", http.HandlerFunc(func (w http.ResponseWriter, r *http.Request) {
//          fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
//    }))
func Handler(rootpath string, h http.Handler, options ...interface{}) *App {
	BeeApp.Handlers.Handler(rootpath, h, options...)
	return BeeApp
}

// InsertFilter adds a FilterFunc with pattern condition and action constant.
// The pos means action constant including
// beego.BeforeStatic, beego.BeforeRouter, beego.BeforeExec, beego.AfterExec and beego.FinishRouter.
// The bool params is for setting the returnOnOutput value (false allows multiple filters to execute)
func InsertFilter(pattern string, pos int, filter FilterFunc, params ...bool) *App {
	BeeApp.Handlers.InsertFilter(pattern, pos, filter, params...)
	return BeeApp
}
