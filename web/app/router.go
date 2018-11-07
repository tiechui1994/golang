package app

import (
	"fmt"
	"net/http"
	"path"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	beecontext "github.com/astaxie/beego/context"
	"github.com/astaxie/beego/context/param"
	"github.com/astaxie/beego/logs"
	"github.com/astaxie/beego/toolbox"
	"github.com/astaxie/beego/utils"
)

/*
 路由器, beego的核心组件之一
*/

// 默认的过滤器执行点
const (
	BeforeStatic = iota
	BeforeRouter
	BeforeExec
	AfterExec
	FinishRouter
)

// 路由类型
const (
	routerTypeBeego   = iota
	routerTypeRESTFul
	routerTypeHandler
)

var (
	// HTTP的请求类型
	HTTPMETHOD = map[string]bool{
		"GET":       true,
		"POST":      true,
		"PUT":       true,
		"DELETE":    true,
		"PATCH":     true,
		"OPTIONS":   true,
		"HEAD":      true,
		"TRACE":     true,
		"CONNECT":   true,
		"MKCOL":     true,
		"COPY":      true,
		"MOVE":      true,
		"PROPFIND":  true,
		"PROPPATCH": true,
		"LOCK":      true,
		"UNLOCK":    true,
	}

	// beego.Controller 默认自带的方法
	exceptMethod = []string{"Init", "Prepare", "Finish", "Render", "RenderString",
		"RenderBytes", "Redirect", "Abort", "StopRun", "UrlFor", "ServeJSON", "ServeJSONP",
		"ServeYAML", "ServeXML", "Input", "ParseForm", "GetString", "GetStrings", "GetInt", "GetBool",
		"GetFloat", "GetFile", "SaveToFile", "StartSession", "SetSession", "GetSession",
		"DelSession", "SessionRegenerateID", "DestroySession", "IsAjax", "GetSecureCookie",
		"SetSecureCookie", "XsrfToken", "CheckXsrfCookie", "XsrfFormHtml",
		"GetControllerAndAction", "ServeFormatted"}

	urlPlaceholder = "{{placeholder}}"
	// 如果返回true,DefaultAccessLogFilter将跳过accesslog
	DefaultAccessLogFilter FilterHandler = &logFilter{}
)

type FilterHandler interface {
	Filter(*beecontext.Context) bool
}

// 日志过滤器: 在某些特殊请求和静态资源请求不需要在access当中记录
type logFilter struct{}

func (l *logFilter) Filter(ctx *beecontext.Context) bool {
	requestPath := path.Clean(ctx.Request.URL.Path) // 获取同目录的最短路径
	// 特殊请求
	if requestPath == "/favicon.ico" || requestPath == "/robots.txt" {
		return true
	}

	// 静态资源请求
	for prefix := range BConfig.WebConfig.StaticDir {
		if strings.HasPrefix(requestPath, prefix) {
			return true
		}
	}

	return false
}

//------------------------------------------------------------------------------------------

// 增加Controller方法, 这些方法不需要通过反射获取
func ExceptMethodAppend(action string) {
	exceptMethod = append(exceptMethod, action)
}

// 存储Controller的一些信息
type ControllerInfo struct {
	pattern        string            // 路由
	controllerType reflect.Type      // 通过此参数可以获取到Controller的所有方法
	methods        map[string]string // 请求类型 : 方法名称
	handler        http.Handler      // request, response 为参数的函数
	runFunction    FilterFunc        // context为参数的函数
	routerType     int
	initialize     func() ControllerInterface // 初始化函数(可选)
	methodParams   []*param.MethodParam       // 方法参数(可选)
}

// Controller容器, 存储注册的router rule, controller handler and filter
type ControllerRegister struct {
	routers      map[string]*Tree // 请求类型 : 路由Tree
	enablePolicy bool
	policies     map[string]*Tree // ???
	enableFilter bool
	filters      [FinishRouter + 1][]*FilterRouter
	pool         sync.Pool // Context池
}

func NewControllerRegister() *ControllerRegister {
	cr := &ControllerRegister{
		routers:  make(map[string]*Tree),
		policies: make(map[string]*Tree),
	}
	cr.pool.New = func() interface{} {
		return beecontext.NewContext()
	}
	return cr
}

/*
	向ControllerRegister当中添加路由
	Add("/user", &UserController{})
	Add("/api/list", &RestController{}, "*:ListFood")
	Add("/api/create", &RestController{}, "post:CreateFood")
	Add("/api/update", &RestController{}, "put:UpdateFood")
	Add("/api/delete", &RestController{}, "delete:DeleteFood")
	Add("/api", &RestController{}, "get,post:ApiFunc"
	Add("/simple", &SimpleController{}, "get:GetFunc;post:PostFunc")
*/
func (p *ControllerRegister) Add(pattern string, c ControllerInterface, mappingMethods ...string) {
	p.addWithMethodParams(pattern, c, nil, mappingMethods...)
}

/*
向Controller当中添加Method
pattern: url路由
c: Controller实例, 必须是一个指针实例
methodParams: 参数
mappingMethods: 方法映射, "TYPE:METHOD", 例如: "post:postFunc;get,post:func"
*/
func (p *ControllerRegister) addWithMethodParams(pattern string, controller ControllerInterface, methodParams []*param.MethodParam, mappingMethods ...string) {
	/*
	  value是一个指针,这里获取了该指针指向的值,相当于value.Elem()
	  value = reflect.Indirect(value)
	*/

	ctrVal := reflect.ValueOf(controller)      // 获取c的真实对象(该对象实现了ControllerInterface)
	ctrType := reflect.Indirect(ctrVal).Type() // 获取c的真实Struct
	methods := make(map[string]string)         // 请求类型:方法名称

	if len(mappingMethods) > 0 {
		semi := strings.Split(mappingMethods[0], ";") // 切割产生映射
		for _, v := range semi {
			colon := strings.Split(v, ":") // 切割产生请求类型, 方法
			if len(colon) != 2 {
				panic("method mapping format is invalid") // 映射混乱
			}
			comma := strings.Split(colon[0], ",") // 一个方法共用多个请求类型
			for _, m := range comma {
				if m == "*" || HTTPMETHOD[strings.ToUpper(m)] {
					if val := ctrVal.MethodByName(colon[1]); val.IsValid() {
						methods[strings.ToUpper(m)] = colon[1]
					} else {
						panic("'" + colon[1] + "' method doesn't exist in the controller " + t.Name()) // Controller的私有方法
					}
				} else {
					panic(v + " is an invalid method mapping. Method doesn't exist " + m) // 请求类型不存在, 或请求类型非法
				}
			}
		}
	}

	// 构建ControllerInfo
	route := &ControllerInfo{}
	route.pattern = pattern
	route.methods = methods
	route.routerType = routerTypeBeego
	route.controllerType = ctrType

	// 创建一个执行的执行的Controller实例
	route.initialize = func() ControllerInterface {
		execType := reflect.New(route.controllerType)
		execController, ok := execType.Interface().(ControllerInterface) // 获取真实的Controller实例
		if !ok {
			panic("controller is not ControllerInterface")
		}

		elemVal := reflect.ValueOf(controller).Elem()      // Controller值
		elemType := reflect.TypeOf(controller).Elem()      // Controller类型
		execElem := reflect.ValueOf(execController).Elem() // 执行Controller

		numOfFields := elemVal.NumField()
		for i := 0; i < numOfFields; i++ {
			fieldType := elemType.Field(i)
			elemField := execElem.FieldByName(fieldType.Name) // 获取Field
			if elemField.CanSet() {
				fieldVal := elemVal.Field(i)
				elemField.Set(fieldVal)
			}
		}

		return execController
	}

	route.methodParams = methodParams
	if len(methods) == 0 { // Controller没有添加新的方法
		for m := range HTTPMETHOD {
			p.addToRouter(m, pattern, route)
		}
	} else { // Controller添加了新的方法
		for k := range methods {
			if k == "*" {
				for m := range HTTPMETHOD {
					p.addToRouter(m, pattern, route)
				}
			} else {
				p.addToRouter(k, pattern, route)
			}
		}
	}
}

// 添加路由
// method: 为请求类型
// pattern: 路由
// r: 存储Controller有效信息
func (p *ControllerRegister) addToRouter(method, pattern string, r *ControllerInfo) {
	// 路由不忽略大小写时, 路由全部保存为小写
	if !BConfig.RouterCaseSensitive {
		pattern = strings.ToLower(pattern)
	}

	if t, ok := p.routers[method]; ok {
		t.AddRouter(pattern, r)
	} else {
		tree := NewTree()
		tree.AddRouter(pattern, r)
		p.routers[method] = tree
	}
}

// 对于运行模式是DEV, 会自动生成路由文件 router/auto.go
// Include(&BankAccount{}, &OrderController{},&RefundController{},&ReceiptController{})
func (p *ControllerRegister) Include(ctrList ...ControllerInterface) {
	if BConfig.RunMode == DEV {
		skip := make(map[string]bool, 10)
		for _, c := range ctrList {
			ctrVal := reflect.ValueOf(c)
			ctrType := reflect.Indirect(ctrVal).Type()

			wgopath := utils.GetGOPATHs() // 获取GOPATH, 可能是多个目录
			if len(wgopath) == 0 {
				panic("you are in dev mode. So please set gopath")
			}
			pkgpath := ""
			for _, wg := range wgopath {
				// filepath.EvalSymlinks() 判断文件或文件夹是否存在
				wg, _ = filepath.EvalSymlinks(filepath.Join(wg, "src", ctrType.PkgPath())) // 获取controller的路径
				if utils.FileExists(wg) {
					pkgpath = wg
					break
				}
			}

			if pkgpath != "" {
				if _, ok := skip[pkgpath]; !ok {
					skip[pkgpath] = true
					parserPkg(pkgpath, ctrType.PkgPath()) // 解析ctr, 路由生成
				}
			}
		}
	}

	for _, c := range ctrList {
		ctrVal := reflect.ValueOf(c)
		ctrType := reflect.Indirect(ctrVal).Type()
		key := ctrType.PkgPath() + ":" + ctrType.Name()
		if comm, ok := GlobalControllerRouter[key]; ok {
			for _, a := range comm {
				p.addWithMethodParams(a.Router, c, a.MethodParams, strings.Join(a.AllowHTTPMethods, ",")+":"+a.Method)
			}
		}
	}
}

func (p *ControllerRegister) Get(pattern string, f FilterFunc) {
	p.AddMethod("get", pattern, f)
}

func (p *ControllerRegister) Post(pattern string, f FilterFunc) {
	p.AddMethod("post", pattern, f)
}

func (p *ControllerRegister) Put(pattern string, f FilterFunc) {
	p.AddMethod("put", pattern, f)
}

func (p *ControllerRegister) Delete(pattern string, f FilterFunc) {
	p.AddMethod("delete", pattern, f)
}

func (p *ControllerRegister) Head(pattern string, f FilterFunc) {
	p.AddMethod("head", pattern, f)
}

func (p *ControllerRegister) Patch(pattern string, f FilterFunc) {
	p.AddMethod("patch", pattern, f)
}

func (p *ControllerRegister) Options(pattern string, f FilterFunc) {
	p.AddMethod("options", pattern, f)
}

func (p *ControllerRegister) Any(pattern string, f FilterFunc) {
	p.AddMethod("*", pattern, f)
}

// 添加路由: request, response
func (p *ControllerRegister) AddMethod(method, pattern string, f FilterFunc) {
	method = strings.ToUpper(method)
	if method != "*" && !HTTPMETHOD[method] {
		panic("not support http method: " + method)
	}

	// 构建路由
	route := &ControllerInfo{}
	route.pattern = pattern
	route.routerType = routerTypeRESTFul
	route.runFunction = f
	methods := make(map[string]string)
	if method == "*" {
		for val := range HTTPMETHOD {
			methods[val] = val
		}
	} else {
		methods[method] = method
	}
	route.methods = methods

	// 路由注册
	for k := range methods {
		if k == "*" {
			for m := range HTTPMETHOD {
				p.addToRouter(m, pattern, route)
			}
		} else {
			p.addToRouter(k, pattern, route)
		}
	}
}

// 添加路由: Context
func (p *ControllerRegister) Handler(pattern string, h http.Handler, options ...interface{}) {
	route := &ControllerInfo{}
	route.pattern = pattern
	route.routerType = routerTypeHandler
	route.handler = h

	// options是补充路由
	if len(options) > 0 {
		if _, ok := options[0].(bool); ok {
			pattern = path.Join(pattern, "?:all(.*)")
		}
	}

	// 这里的method是全部的HTTPMETHOD
	for m := range HTTPMETHOD {
		p.addToRouter(m, pattern, route)
	}
}

/*
 自动添加路由:
 例如: beego.AddAuto(&MainContorlller{}), MainController有方法List()和Page().
 访问 url /main/list 则执行MainController的List()方法,
 访问 url /main/page 则执行MainController的Page()方法.
*/
func (p *ControllerRegister) AddAuto(c ControllerInterface) {
	p.AddAutoPrefix("/", c)
}

/*
自动添加路由:
	prefix是路由前缀
	controller是Controller实例
	例如: beego.AddAutoPrefix("/admin",&MainContorlller{}), MainContorlller有方法
	List()和Page()

	请求/admin/main/list, 则执行MainController的List()
	请求/admin/main/page, 则执行MainController的Page()
*/
func (p *ControllerRegister) AddAutoPrefix(prefix string, controller ControllerInterface) {
	ctrVal := reflect.ValueOf(controller)
	ct := reflect.Indirect(ctrVal).Type() // 获取controller的Type
	rt := ctrVal.Type()
	controllerName := strings.TrimSuffix(ct.Name(), "Controller")
	for i := 0; i < rt.NumMethod(); i++ {
		if !utils.InSlice(rt.Method(i).Name, exceptMethod) { // 去除掉exceptMethod的方法
			route := &ControllerInfo{}
			route.routerType = routerTypeBeego
			route.methods = map[string]string{"*": rt.Method(i).Name}
			route.controllerType = ct
			// 路由: "/{prefix}/{controller}/{method}/*"
			pattern := path.Join(prefix, strings.ToLower(controllerName), strings.ToLower(rt.Method(i).Name), "*")
			patternInit := path.Join(prefix, controllerName, rt.Method(i).Name, "*")
			patternFix := path.Join(prefix, strings.ToLower(controllerName), strings.ToLower(rt.Method(i).Name))
			patternFixInit := path.Join(prefix, controllerName, rt.Method(i).Name)
			route.pattern = pattern
			for m := range HTTPMETHOD {
				p.addToRouter(m, pattern, route)
				p.addToRouter(m, patternInit, route)
				p.addToRouter(m, patternFix, route)
				p.addToRouter(m, patternFixInit, route)
			}
		}
	}
}

// 插入拦截器
// params:
//   1. 设置returnOnOutput值 (false 允许多个filter执行)
//   2. 确定是否需要重置参数.
func (p *ControllerRegister) InsertFilter(pattern string, position int, filter FilterFunc, params ...bool) error {
	mr := &FilterRouter{
		tree:           NewTree(),
		pattern:        pattern,
		filterFunc:     filter,
		returnOnOutput: true,
	}

	if !BConfig.RouterCaseSensitive {
		mr.pattern = strings.ToLower(pattern)
	}

	paramsLen := len(params)
	if paramsLen > 0 {
		mr.returnOnOutput = params[0]
	}
	if paramsLen > 1 {
		mr.resetParams = params[1]
	}
	mr.tree.AddRouter(pattern, true)
	return p.insertFilterRouter(position, mr)
}

// 插入操作
func (p *ControllerRegister) insertFilterRouter(position int, mr *FilterRouter) (err error) {
	if position < BeforeStatic || position > FinishRouter {
		err = fmt.Errorf("can not find your filter position")
		return
	}

	p.enableFilter = true                                 // 激活路由器
	p.filters[position] = append(p.filters[position], mr) // 在指定的拦截点添加拦截器
	return nil
}

// 请求转发(在一个Handler当中调用其他的Handler)
// endpoint: {path}.{controller}.{method}
// values: "key", "value"
func (p *ControllerRegister) URLFor(endpoint string, values ...interface{}) string {
	paths := strings.Split(endpoint, ".")
	if len(paths) <= 1 {
		logs.Warn("urlfor endpoint must like path.controller.method")
		return ""
	}
	if len(values)%2 != 0 {
		logs.Warn("urlfor params must key-value pair")
		return ""
	}
	params := make(map[string]string)
	if len(values) > 0 {
		key := ""
		for k, v := range values {
			if k%2 == 0 {
				key = fmt.Sprint(v)
			} else {
				params[key] = fmt.Sprint(v)
			}
		}
	}

	controllName := strings.Join(paths[:len(paths)-1], "/")
	methodName := paths[len(paths)-1]
	for m, t := range p.routers {
		ok, url := p.geturl(t, "/", controllName, methodName, params, m)
		if ok {
			return url
		}
	}
	return ""
}

func (p *ControllerRegister) geturl(t *Tree, url, controllName, methodName string, params map[string]string, httpMethod string) (bool, string) {
	for _, subtree := range t.fixrouters {
		u := path.Join(url, subtree.prefix)
		ok, u := p.geturl(subtree, u, controllName, methodName, params, httpMethod)
		if ok {
			return ok, u
		}
	}
	if t.wildcard != nil {
		u := path.Join(url, urlPlaceholder)
		ok, u := p.geturl(t.wildcard, u, controllName, methodName, params, httpMethod)
		if ok {
			return ok, u
		}
	}
	for _, l := range t.leaves {
		if c, ok := l.runObject.(*ControllerInfo); ok {
			if c.routerType == routerTypeBeego &&
				strings.HasSuffix(path.Join(c.controllerType.PkgPath(), c.controllerType.Name()), controllName) {
				find := false
				if HTTPMETHOD[strings.ToUpper(methodName)] {
					if len(c.methods) == 0 {
						find = true
					} else if m, ok := c.methods[strings.ToUpper(methodName)]; ok && m == strings.ToUpper(methodName) {
						find = true
					} else if m, ok = c.methods["*"]; ok && m == methodName {
						find = true
					}
				}
				if !find {
					for m, md := range c.methods {
						if (m == "*" || m == httpMethod) && md == methodName {
							find = true
						}
					}
				}
				if find {
					if l.regexps == nil {
						if len(l.wildcards) == 0 {
							return true, strings.Replace(url, "/"+urlPlaceholder, "", 1) + toURL(params)
						}
						if len(l.wildcards) == 1 {
							if v, ok := params[l.wildcards[0]]; ok {
								delete(params, l.wildcards[0])
								return true, strings.Replace(url, urlPlaceholder, v, 1) + toURL(params)
							}
							return false, ""
						}
						if len(l.wildcards) == 3 && l.wildcards[0] == "." {
							if p, ok := params[":path"]; ok {
								if e, isok := params[":ext"]; isok {
									delete(params, ":path")
									delete(params, ":ext")
									return true, strings.Replace(url, urlPlaceholder, p+"."+e, -1) + toURL(params)
								}
							}
						}
						canskip := false
						for _, v := range l.wildcards {
							if v == ":" {
								canskip = true
								continue
							}
							if u, ok := params[v]; ok {
								delete(params, v)
								url = strings.Replace(url, urlPlaceholder, u, 1)
							} else {
								if canskip {
									canskip = false
									continue
								}
								return false, ""
							}
						}
						return true, url + toURL(params)
					}
					var i int
					var startreg bool
					regurl := ""
					for _, v := range strings.Trim(l.regexps.String(), "^$") {
						if v == '(' {
							startreg = true
							continue
						} else if v == ')' {
							startreg = false
							if v, ok := params[l.wildcards[i]]; ok {
								delete(params, l.wildcards[i])
								regurl = regurl + v
								i++
							} else {
								break
							}
						} else if !startreg {
							regurl = string(append([]rune(regurl), v))
						}
					}
					if l.regexps.MatchString(regurl) {
						ps := strings.Split(regurl, "/")
						for _, p := range ps {
							url = strings.Replace(url, urlPlaceholder, p, 1)
						}
						return true, url + toURL(params)
					}
				}
			}
		}
	}

	return false, ""
}

func (p *ControllerRegister) execFilter(context *beecontext.Context, urlPath string, pos int) (started bool) {
	var preFilterParams map[string]string
	for _, filterR := range p.filters[pos] {
		if filterR.returnOnOutput && context.ResponseWriter.Started {
			return true
		}
		if filterR.resetParams {
			preFilterParams = context.Input.Params()
		}
		if ok := filterR.ValidRouter(urlPath, context); ok {
			filterR.filterFunc(context)
			if filterR.resetParams {
				context.Input.ResetParams()
				for k, v := range preFilterParams {
					context.Input.SetParam(k, v)
				}
			}
		}
		if filterR.returnOnOutput && context.ResponseWriter.Started {
			return true
		}
	}
	return false
}

// Implement http.Handler interface.
func (p *ControllerRegister) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	startTime := time.Now()
	var (
		runRouter    reflect.Type
		findRouter   bool
		runMethod    string
		methodParams []*param.MethodParam
		routerInfo   *ControllerInfo
		isRunnable   bool
	)
	context := p.pool.Get().(*beecontext.Context)
	context.Reset(rw, r)

	defer p.pool.Put(context)
	if BConfig.RecoverFunc != nil {
		defer BConfig.RecoverFunc(context)
	}

	context.Output.EnableGzip = BConfig.EnableGzip

	if BConfig.RunMode == DEV {
		context.Output.Header("Server", BConfig.ServerName)
	}

	var urlPath = r.URL.Path

	if !BConfig.RouterCaseSensitive {
		urlPath = strings.ToLower(urlPath)
	}

	// filter wrong http method
	if !HTTPMETHOD[r.Method] {
		http.Error(rw, "Method Not Allowed", 405)
		goto Admin
	}

	// filter for static file
	if len(p.filters[BeforeStatic]) > 0 && p.execFilter(context, urlPath, BeforeStatic) {
		goto Admin
	}

	serverStaticRouter(context)

	if context.ResponseWriter.Started {
		findRouter = true
		goto Admin
	}

	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		if BConfig.CopyRequestBody && !context.Input.IsUpload() {
			context.Input.CopyBody(BConfig.MaxMemory)
		}
		context.Input.ParseFormOrMulitForm(BConfig.MaxMemory)
	}

	// session init
	if BConfig.WebConfig.Session.SessionOn {
		var err error
		context.Input.CruSession, err = GlobalSessions.SessionStart(rw, r)
		if err != nil {
			logs.Error(err)
			exception("503", context)
			goto Admin
		}
		defer func() {
			if context.Input.CruSession != nil {
				context.Input.CruSession.SessionRelease(rw)
			}
		}()
	}
	if len(p.filters[BeforeRouter]) > 0 && p.execFilter(context, urlPath, BeforeRouter) {
		goto Admin
	}
	// User can define RunController and RunMethod in filter
	if context.Input.RunController != nil && context.Input.RunMethod != "" {
		findRouter = true
		runMethod = context.Input.RunMethod
		runRouter = context.Input.RunController
	} else {
		routerInfo, findRouter = p.FindRouter(context)
	}

	//if no matches to url, throw a not found exception
	if !findRouter {
		exception("404", context)
		goto Admin
	}
	if splat := context.Input.Param(":splat"); splat != "" {
		for k, v := range strings.Split(splat, "/") {
			context.Input.SetParam(strconv.Itoa(k), v)
		}
	}

	//execute middleware filters
	if len(p.filters[BeforeExec]) > 0 && p.execFilter(context, urlPath, BeforeExec) {
		goto Admin
	}

	//check policies
	if p.execPolicy(context, urlPath) {
		goto Admin
	}

	if routerInfo != nil {
		//store router pattern into context
		context.Input.SetData("RouterPattern", routerInfo.pattern)
		if routerInfo.routerType == routerTypeRESTFul {
			if _, ok := routerInfo.methods[r.Method]; ok {
				isRunnable = true
				routerInfo.runFunction(context)
			} else {
				exception("405", context)
				goto Admin
			}
		} else if routerInfo.routerType == routerTypeHandler {
			isRunnable = true
			routerInfo.handler.ServeHTTP(rw, r)
		} else {
			runRouter = routerInfo.controllerType
			methodParams = routerInfo.methodParams
			method := r.Method
			if r.Method == http.MethodPost && context.Input.Query("_method") == http.MethodPost {
				method = http.MethodPut
			}
			if r.Method == http.MethodPost && context.Input.Query("_method") == http.MethodDelete {
				method = http.MethodDelete
			}
			if m, ok := routerInfo.methods[method]; ok {
				runMethod = m
			} else if m, ok = routerInfo.methods["*"]; ok {
				runMethod = m
			} else {
				runMethod = method
			}
		}
	}

	// also defined runRouter & runMethod from filter
	if !isRunnable {
		//Invoke the request handler
		var execController ControllerInterface
		if routerInfo.initialize != nil {
			execController = routerInfo.initialize()
		} else {
			vc := reflect.New(runRouter)
			var ok bool
			execController, ok = vc.Interface().(ControllerInterface)
			if !ok {
				panic("controller is not ControllerInterface")
			}
		}

		//call the controller init function
		execController.Init(context, runRouter.Name(), runMethod, execController)

		//call prepare function
		execController.Prepare()

		//if XSRF is Enable then check cookie where there has any cookie in the  request's cookie _csrf
		if BConfig.WebConfig.EnableXSRF {
			execController.XSRFToken()
			if r.Method == http.MethodPost || r.Method == http.MethodDelete || r.Method == http.MethodPut ||
				(r.Method == http.MethodPost && (context.Input.Query("_method") == http.MethodDelete || context.Input.Query("_method") == http.MethodPut)) {
				execController.CheckXSRFCookie()
			}
		}

		execController.URLMapping()

		if !context.ResponseWriter.Started {
			//exec main logic
			switch runMethod {
			case http.MethodGet:
				execController.Get()
			case http.MethodPost:
				execController.Post()
			case http.MethodDelete:
				execController.Delete()
			case http.MethodPut:
				execController.Put()
			case http.MethodHead:
				execController.Head()
			case http.MethodPatch:
				execController.Patch()
			case http.MethodOptions:
				execController.Options()
			default:
				if !execController.HandlerFunc(runMethod) {
					vc := reflect.ValueOf(execController)
					method := vc.MethodByName(runMethod)
					in := param.ConvertParams(methodParams, method.Type(), context)
					out := method.Call(in)

					//For backward compatibility we only handle response if we had incoming methodParams
					if methodParams != nil {
						p.handleParamResponse(context, execController, out)
					}
				}
			}

			//render template
			if !context.ResponseWriter.Started && context.Output.Status == 0 {
				if BConfig.WebConfig.AutoRender {
					if err := execController.Render(); err != nil {
						logs.Error(err)
					}
				}
			}
		}

		// finish all runRouter. release resource
		execController.Finish()
	}

	//execute middleware filters
	if len(p.filters[AfterExec]) > 0 && p.execFilter(context, urlPath, AfterExec) {
		goto Admin
	}

	if len(p.filters[FinishRouter]) > 0 && p.execFilter(context, urlPath, FinishRouter) {
		goto Admin
	}

Admin:
//admin module record QPS

	statusCode := context.ResponseWriter.Status
	if statusCode == 0 {
		statusCode = 200
	}

	logAccess(context, &startTime, statusCode)

	if BConfig.Listen.EnableAdmin {
		timeDur := time.Since(startTime)
		pattern := ""
		if routerInfo != nil {
			pattern = routerInfo.pattern
		}

		if FilterMonitorFunc(r.Method, r.URL.Path, timeDur, pattern, statusCode) {
			if runRouter != nil {
				go toolbox.StatisticsMap.AddStatistics(r.Method, r.URL.Path, runRouter.Name(), timeDur)
			} else {
				go toolbox.StatisticsMap.AddStatistics(r.Method, r.URL.Path, "", timeDur)
			}
		}
	}

	if BConfig.RunMode == DEV && !BConfig.Log.AccessLogs {
		var devInfo string
		timeDur := time.Since(startTime)
		iswin := (runtime.GOOS == "windows")
		statusColor := logs.ColorByStatus(iswin, statusCode)
		methodColor := logs.ColorByMethod(iswin, r.Method)
		resetColor := logs.ColorByMethod(iswin, "")
		if findRouter {
			if routerInfo != nil {
				devInfo = fmt.Sprintf("|%15s|%s %3d %s|%13s|%8s|%s %-7s %s %-3s   r:%s", context.Input.IP(), statusColor, statusCode,
					resetColor, timeDur.String(), "match", methodColor, r.Method, resetColor, r.URL.Path,
					routerInfo.pattern)
			} else {
				devInfo = fmt.Sprintf("|%15s|%s %3d %s|%13s|%8s|%s %-7s %s %-3s", context.Input.IP(), statusColor, statusCode, resetColor,
					timeDur.String(), "match", methodColor, r.Method, resetColor, r.URL.Path)
			}
		} else {
			devInfo = fmt.Sprintf("|%15s|%s %3d %s|%13s|%8s|%s %-7s %s %-3s", context.Input.IP(), statusColor, statusCode, resetColor,
				timeDur.String(), "nomatch", methodColor, r.Method, resetColor, r.URL.Path)
		}
		if iswin {
			logs.W32Debug(devInfo)
		} else {
			logs.Debug(devInfo)
		}
	}
	// Call WriteHeader if status code has been set changed
	if context.Output.Status != 0 {
		context.ResponseWriter.WriteHeader(context.Output.Status)
	}
}

func (p *ControllerRegister) handleParamResponse(context *beecontext.Context, execController ControllerInterface, results []reflect.Value) {
	//looping in reverse order for the case when both error and value are returned and error sets the response status code
	for i := len(results) - 1; i >= 0; i-- {
		result := results[i]
		if result.Kind() != reflect.Interface || !result.IsNil() {
			resultValue := result.Interface()
			context.RenderMethodResult(resultValue)
		}
	}
	if !context.ResponseWriter.Started && len(results) > 0 && context.Output.Status == 0 {
		context.Output.SetStatus(200)
	}
}

// FindRouter Find Router info for URL
func (p *ControllerRegister) FindRouter(context *beecontext.Context) (routerInfo *ControllerInfo, isFind bool) {
	var urlPath = context.Input.URL()
	if !BConfig.RouterCaseSensitive {
		urlPath = strings.ToLower(urlPath)
	}
	httpMethod := context.Input.Method()
	if t, ok := p.routers[httpMethod]; ok {
		runObject := t.Match(urlPath, context)
		if r, ok := runObject.(*ControllerInfo); ok {
			return r, true
		}
	}
	return
}

func toURL(params map[string]string) string {
	if len(params) == 0 {
		return ""
	}
	u := "?"
	for k, v := range params {
		u += k + "=" + v + "&"
	}
	return strings.TrimRight(u, "&")
}

func logAccess(ctx *beecontext.Context, startTime *time.Time, statusCode int) {
	//Skip logging if AccessLogs config is false
	if !BConfig.Log.AccessLogs {
		return
	}
	//Skip logging static requests unless EnableStaticLogs config is true
	if !BConfig.Log.EnableStaticLogs && DefaultAccessLogFilter.Filter(ctx) {
		return
	}
	var (
		requestTime time.Time
		elapsedTime time.Duration
		r           = ctx.Request
	)
	if startTime != nil {
		requestTime = *startTime
		elapsedTime = time.Since(*startTime)
	}
	record := &logs.AccessLogRecord{
		RemoteAddr:     ctx.Input.IP(),
		RequestTime:    requestTime,
		RequestMethod:  r.Method,
		Request:        fmt.Sprintf("%s %s %s", r.Method, r.RequestURI, r.Proto),
		ServerProtocol: r.Proto,
		Host:           r.Host,
		Status:         statusCode,
		ElapsedTime:    elapsedTime,
		HTTPReferrer:   r.Header.Get("Referer"),
		HTTPUserAgent:  r.Header.Get("User-Agent"),
		RemoteUser:     r.Header.Get("Remote-User"),
		BodyBytesSent:  0, //@todo this one is missing!
	}
	logs.AccessLog(record, BConfig.Log.AccessLogsFormat)
}
