package app

import "github.com/astaxie/beego/context"

// FilterFunc: 过滤器,在controller的handler之前执行
type FilterFunc func(*context.Context)

type FilterRouter struct {
	filterFunc     FilterFunc // 拦截函数
	tree           *Tree
	pattern        string
	returnOnOutput bool // true表示只允许一个filter执行, false表示允许多个filter执行
	resetParams    bool // 是否重置参数
}

/*
 ValidRouter检查当前请求是否与此过滤器匹配.
 如果请求匹配,则还返回由过滤器模式定义的URL参数的值.
*/
func (f *FilterRouter) ValidRouter(url string, ctx *context.Context) bool {
	isOk := f.tree.Match(url, ctx)
	if isOk != nil {
		if b, ok := isOk.(bool); ok {
			return b
		}
	}
	return false
}
