package app

import (
	"path"
	"regexp"
	"strings"

	"github.com/astaxie/beego/context"
	"github.com/astaxie/beego/utils"
)

var (
	allowSuffixExt = []string{".json", ".xml", ".html"}
)

/*
 Tree有三种元素: FixRouter / wildcard / leaves
 fixRouter存储固定路由器
 wildcard存储参数
 leaves存储端点信息
*/
type Tree struct {
	prefix     string      // 静态路由
	fixrouters []*Tree     // 首先匹配的是固定路由
	wildcard   *Tree       // 当fixrouters匹配失败, 搜索wildcard
	leaves     []*leafInfo // 当wildcard匹配失败, 搜索 leaves
}

func NewTree() *Tree {
	return &Tree{}
}

// AddTree will add tree to the exist Tree
// prefix是一个没有参数的路由
func (t *Tree) AddTree(prefix string, tree *Tree) {
	t.addTree(splitPath(prefix), tree, nil, "")
}

/*
参数说明:
 	segments: 路由使用"/"切分后的数组
 	tree: 目标Tree
 	wildcards: 参数
 	reg:
*/
func (t *Tree) addTree(segments []string, tree *Tree, wildcards []string, reg string) {
	if len(segments) == 0 {
		panic("prefix should has path")
	}
	seg := segments[0]
	iswild, params, regexpStr := splitSegment(seg)

	// 以 ? 开头, 忽略之
	if len(params) > 0 && params[0] == ":" {
		params = params[1:]
		if len(segments[1:]) > 0 {
			t.addTree(segments[1:], tree, append(wildcards, params...), reg)
		} else {
			filterTreeWithPrefix(tree, wildcards, reg)
		}
	}

	// Rule: /login/*/access 可以匹配 /login/2009/11/access
	//
	// if already has *, and when loop the access, should as a regexpStr
	if !iswild && utils.InSlice(":splat", wildcards) {
		iswild = true
		regexpStr = seg
	}

	// Rule: /user/:id/*
	if seg == "*" && len(wildcards) > 0 && reg == "" {
		regexpStr = "(.+)"
	}

	// 只用一个segment
	if len(segments) == 1 {
		if iswild {
			if regexpStr != "" {
				if reg == "" {
					rr := ""
					for _, w := range wildcards {
						if w == ":splat" {
							rr = rr + "(.+)/"
						} else {
							rr = rr + "([^/]+)/"
						}
					}
					regexpStr = rr + regexpStr
				} else {
					regexpStr = "/" + regexpStr
				}
			} else if reg != "" {
				if seg == "*.*" {
					regexpStr = "([^.]+).(.+)"
				} else {
					for _, w := range params {
						if w == "." || w == ":" {
							continue
						}
						regexpStr = "([^/]+)/" + regexpStr
					}
				}
			}
			reg = strings.Trim(reg+"/"+regexpStr, "/")
			filterTreeWithPrefix(tree, append(wildcards, params...), reg)
			t.wildcard = tree
		} else {
			reg = strings.Trim(reg+"/"+regexpStr, "/")
			filterTreeWithPrefix(tree, append(wildcards, params...), reg)
			tree.prefix = seg
			t.fixrouters = append(t.fixrouters, tree)
		}
		return
	}

	if iswild {
		if t.wildcard == nil {
			t.wildcard = NewTree()
		}
		if regexpStr != "" {
			if reg == "" {
				rr := ""
				for _, w := range wildcards {
					if w == ":splat" {
						rr = rr + "(.+)/"
					} else {
						rr = rr + "([^/]+)/"
					}
				}
				regexpStr = rr + regexpStr
			} else {
				regexpStr = "/" + regexpStr
			}
		} else if reg != "" {
			if seg == "*.*" {
				regexpStr = "([^.]+).(.+)"
				params = params[1:]
			} else {
				for range params {
					regexpStr = "([^/]+)/" + regexpStr
				}
			}
		} else {
			if seg == "*.*" {
				params = params[1:]
			}
		}
		reg = strings.TrimRight(strings.TrimRight(reg, "/")+"/"+regexpStr, "/")
		t.wildcard.addTree(segments[1:], tree, append(wildcards, params...), reg)
	} else {
		subTree := NewTree()
		subTree.prefix = seg
		t.fixrouters = append(t.fixrouters, subTree)
		subTree.addTree(segments[1:], tree, append(wildcards, params...), reg)
	}
}

func filterTreeWithPrefix(t *Tree, wildcards []string, reg string) {
	for _, v := range t.fixrouters {
		filterTreeWithPrefix(v, wildcards, reg)
	}
	if t.wildcard != nil {
		filterTreeWithPrefix(t.wildcard, wildcards, reg)
	}
	for _, l := range t.leaves {
		if reg != "" {
			if l.regexps != nil {
				l.wildcards = append(wildcards, l.wildcards...)
				l.regexps = regexp.MustCompile("^" + reg + "/" + strings.Trim(l.regexps.String(), "^$") + "$")
			} else {
				for _, v := range l.wildcards {
					if v == ":splat" {
						reg = reg + "/(.+)"
					} else {
						reg = reg + "/([^/]+)"
					}
				}
				l.regexps = regexp.MustCompile("^" + reg + "$")
				l.wildcards = append(wildcards, l.wildcards...)
			}
		} else {
			l.wildcards = append(wildcards, l.wildcards...)
			if l.regexps != nil {
				for _, w := range wildcards {
					if w == ":splat" {
						reg = "(.+)/" + reg
					} else {
						reg = "([^/]+)/" + reg
					}
				}
				l.regexps = regexp.MustCompile("^" + reg + strings.Trim(l.regexps.String(), "^$") + "$")
			}
		}
	}
}

// AddRouter call addseg function
func (t *Tree) AddRouter(pattern string, runObject interface{}) {
	t.addseg(splitPath(pattern), runObject, nil, "")
}

// "/"
// "admin" ->
func (t *Tree) addseg(segments []string, route interface{}, wildcards []string, reg string) {
	if len(segments) == 0 {
		if reg != "" {
			t.leaves = append(t.leaves, &leafInfo{runObject: route, wildcards: wildcards, regexps: regexp.MustCompile("^" + reg + "$")})
		} else {
			t.leaves = append(t.leaves, &leafInfo{runObject: route, wildcards: wildcards})
		}
	} else {
		seg := segments[0]
		iswild, params, regexpStr := splitSegment(seg)
		// if it's ? meaning can igone this, so add one more rule for it
		if len(params) > 0 && params[0] == ":" {
			t.addseg(segments[1:], route, wildcards, reg)
			params = params[1:]
		}
		//Rule: /login/*/access match /login/2009/11/access
		//if already has *, and when loop the access, should as a regexpStr
		if !iswild && utils.InSlice(":splat", wildcards) {
			iswild = true
			regexpStr = seg
		}
		//Rule: /user/:id/*
		if seg == "*" && len(wildcards) > 0 && reg == "" {
			regexpStr = "(.+)"
		}
		if iswild {
			if t.wildcard == nil {
				t.wildcard = NewTree()
			}
			if regexpStr != "" {
				if reg == "" {
					rr := ""
					for _, w := range wildcards {
						if w == ":splat" {
							rr = rr + "(.+)/"
						} else {
							rr = rr + "([^/]+)/"
						}
					}
					regexpStr = rr + regexpStr
				} else {
					regexpStr = "/" + regexpStr
				}
			} else if reg != "" {
				if seg == "*.*" {
					regexpStr = "/([^.]+).(.+)"
					params = params[1:]
				} else {
					for range params {
						regexpStr = "/([^/]+)" + regexpStr
					}
				}
			} else {
				if seg == "*.*" {
					params = params[1:]
				}
			}
			t.wildcard.addseg(segments[1:], route, append(wildcards, params...), reg+regexpStr)
		} else {
			var subTree *Tree
			for _, sub := range t.fixrouters {
				if sub.prefix == seg {
					subTree = sub
					break
				}
			}
			if subTree == nil {
				subTree = NewTree()
				subTree.prefix = seg
				t.fixrouters = append(t.fixrouters, subTree)
			}
			subTree.addseg(segments[1:], route, wildcards, reg)
		}
	}
}

// Match router to runObject & params
func (t *Tree) Match(pattern string, ctx *context.Context) (runObject interface{}) {
	if len(pattern) == 0 || pattern[0] != '/' {
		return nil
	}
	w := make([]string, 0, 20)
	return t.match(pattern[1:], pattern, w, ctx)
}

func (t *Tree) match(treePattern string, pattern string, wildcardValues []string, ctx *context.Context) (runObject interface{}) {
	if len(pattern) > 0 {
		i := 0
		for ; i < len(pattern) && pattern[i] == '/'; i++ {
		}
		pattern = pattern[i:]
	}
	// Handle leaf nodes:
	if len(pattern) == 0 {
		for _, l := range t.leaves {
			if ok := l.match(treePattern, wildcardValues, ctx); ok {
				return l.runObject
			}
		}
		if t.wildcard != nil {
			for _, l := range t.wildcard.leaves {
				if ok := l.match(treePattern, wildcardValues, ctx); ok {
					return l.runObject
				}
			}
		}
		return nil
	}
	var seg string
	i, l := 0, len(pattern)
	for ; i < l && pattern[i] != '/'; i++ {
	}
	if i == 0 {
		seg = pattern
		pattern = ""
	} else {
		seg = pattern[:i]
		pattern = pattern[i:]
	}
	for _, subTree := range t.fixrouters {
		if subTree.prefix == seg {
			if len(pattern) != 0 && pattern[0] == '/' {
				treePattern = pattern[1:]
			} else {
				treePattern = pattern
			}
			runObject = subTree.match(treePattern, pattern, wildcardValues, ctx)
			if runObject != nil {
				break
			}
		}
	}
	if runObject == nil && len(t.fixrouters) > 0 {
		// Filter the .json .xml .html extension
		for _, str := range allowSuffixExt {
			if strings.HasSuffix(seg, str) {
				for _, subTree := range t.fixrouters {
					if subTree.prefix == seg[:len(seg)-len(str)] {
						runObject = subTree.match(treePattern, pattern, wildcardValues, ctx)
						if runObject != nil {
							ctx.Input.SetParam(":ext", str[1:])
						}
					}
				}
			}
		}
	}
	if runObject == nil && t.wildcard != nil {
		runObject = t.wildcard.match(treePattern, pattern, append(wildcardValues, seg), ctx)
	}

	if runObject == nil && len(t.leaves) > 0 {
		wildcardValues = append(wildcardValues, seg)
		start, i := 0, 0
		for ; i < len(pattern); i++ {
			if pattern[i] == '/' {
				if i != 0 && start < len(pattern) {
					wildcardValues = append(wildcardValues, pattern[start:i])
				}
				start = i + 1
				continue
			}
		}
		if start > 0 {
			wildcardValues = append(wildcardValues, pattern[start:i])
		}
		for _, l := range t.leaves {
			if ok := l.match(treePattern, wildcardValues, ctx); ok {
				return l.runObject
			}
		}
	}
	return runObject
}

type leafInfo struct {
	// names of wildcards that lead to this leaf. eg, ["id" "name"] for the wildcard ":id" and ":name"
	wildcards []string

	// if the leaf is regexp
	regexps *regexp.Regexp

	runObject interface{}
}

func (leaf *leafInfo) match(treePattern string, wildcardValues []string, ctx *context.Context) (ok bool) {
	//fmt.Println("Leaf:", wildcardValues, leaf.wildcards, leaf.regexps)
	if leaf.regexps == nil {
		if len(wildcardValues) == 0 && len(leaf.wildcards) == 0 { // static path
			return true
		}
		// match *
		if len(leaf.wildcards) == 1 && leaf.wildcards[0] == ":splat" {
			ctx.Input.SetParam(":splat", treePattern)
			return true
		}
		// match *.* or :id
		if len(leaf.wildcards) >= 2 && leaf.wildcards[len(leaf.wildcards)-2] == ":path" && leaf.wildcards[len(leaf.wildcards)-1] == ":ext" {
			if len(leaf.wildcards) == 2 {
				lastone := wildcardValues[len(wildcardValues)-1]
				strs := strings.SplitN(lastone, ".", 2)
				if len(strs) == 2 {
					ctx.Input.SetParam(":ext", strs[1])
				}
				ctx.Input.SetParam(":path", path.Join(path.Join(wildcardValues[:len(wildcardValues)-1]...), strs[0]))
				return true
			} else if len(wildcardValues) < 2 {
				return false
			}
			var index int
			for index = 0; index < len(leaf.wildcards)-2; index++ {
				ctx.Input.SetParam(leaf.wildcards[index], wildcardValues[index])
			}
			lastone := wildcardValues[len(wildcardValues)-1]
			strs := strings.SplitN(lastone, ".", 2)
			if len(strs) == 2 {
				ctx.Input.SetParam(":ext", strs[1])
			}
			if index > (len(wildcardValues) - 1) {
				ctx.Input.SetParam(":path", "")
			} else {
				ctx.Input.SetParam(":path", path.Join(path.Join(wildcardValues[index:len(wildcardValues)-1]...), strs[0]))
			}
			return true
		}
		// match :id
		if len(leaf.wildcards) != len(wildcardValues) {
			return false
		}
		for j, v := range leaf.wildcards {
			ctx.Input.SetParam(v, wildcardValues[j])
		}
		return true
	}

	if !leaf.regexps.MatchString(path.Join(wildcardValues...)) {
		return false
	}
	matches := leaf.regexps.FindStringSubmatch(path.Join(wildcardValues...))
	for i, match := range matches[1:] {
		if i < len(leaf.wildcards) {
			ctx.Input.SetParam(leaf.wildcards[i], match)
		}
	}
	return true
}

/*
 路径切割:
 "/" -> []
 "/admin" -> ["admin"]
 "/admin/" -> ["admin"]
 "/admin/users" -> ["admin", "users"]
*/
func splitPath(key string) []string {
	key = strings.Trim(key, "/ ")
	if key == "" {
		return []string{}
	}
	return strings.Split(key, "/")
}

/************************************************************************
"admin"        -> false, nil, ""

":id"               -> true, [:id], ""
"?:id"              -> true, [: :id], ""        : meaning can empty
":id:int"           -> true, [:id], ([0-9]+)
":name:string"      -> true, [:name], ([\w]+)
":id([0-9]+)"  	    -> true, [:id], ([0-9]+)
":id([0-9]+)_:name" -> true, [:id :name], ([0-9]+)_(.+)

"cms_:id_:page.html"     -> true, [:id_ :page], cms_(.+)(.+).html
"cms_:id(.+)_:page.html" -> true, [:id :page], cms_(.+)_(.+).html

"*"   -> true, [:splat], ""
"*.*" -> true,[. :path :ext], ""      . meaning separator
*************************************************************************/

/*
返回值说明:
bool, 是否是正则表达式
[]string, 分组数组
string,  正则表达式

思路:
	1. 参数检测(命名参数 | 匿名参数"?")
	2. 转换正则表达式, 以"("开头, 则就为正则表达式; ":"开头, 且后面是int, string, 需要转换;
       其他, 转换为`(.+)xxx`, 其中xxx表示 非[0-9A-Za-z_)]字符
	3. 转向第1步
*/
func splitSegment(key string) (bool, []string, string) {
	// "*" 前缀检查
	if strings.HasPrefix(key, "*") {
		if key == "*.*" {
			return true, []string{".", ":path", ":ext"}, ""
		}
		return true, []string{":splat"}, ""
	}

	// ":" 分隔符检查
	if strings.ContainsAny(key, ":") {
		var (
			skipNum   int // 需要忽略的字母个数,
			paramsNum int // 参数个数

			startCom bool // 通用表达式匹配, ":xxx:int|string"
			startExp bool // 正则表达式, ":xxx(....)"

			param []rune // 参数
			exp   []rune // 记录startExp过程中产生的是正则表达式
			out   []rune // 记录函数返回的正则表达式
		)

		params := make([]string, 0) // 参数数组, 函数返回的[]string
		reg := regexp.MustCompile(`[a-zA-Z0-9_]+`)

		for i, v := range key {
			if skipNum > 0 {
				skipNum--
				continue
			}

			if startCom {
				// 参数匹配, "[a-zA-Z0-9_]"
				if reg.MatchString(string(v)) {
					param = append(param, v)
					continue
				}

				// 通用字符串匹配; 一个匹配回合的匹配结束
				if v == ':' {
					// int 匹配
					if len(key) >= i+4 && key[i+1:i+4] == "int" {
						out = append(out, []rune("([0-9]+)")...)
						params = append(params, ":"+string(param))
						startCom = false
						startExp = false
						skipNum = 3
						param = make([]rune, 0)
						paramsNum++
						continue
					}

					// string 匹配
					if len(key) >= i+7 && key[i+1:i+7] == "string" {
						out = append(out, []rune(`([\w]+)`)...)
						params = append(params, ":"+string(param))
						paramsNum++
						startCom = false
						startExp = false
						skipNum = 6
						param = make([]rune, 0)
						continue
					}
				}

				// 其他, 不是通用字符串, 也不是正则表达式; 一个匹配回合的匹配结束
				if v != '(' {
					out = append(out, []rune(`(.+)`)...)
					params = append(params, ":"+string(param))
					param = make([]rune, 0)
					paramsNum++
					startCom = false
					startExp = false
				}
			}

			if startExp {
				// 正则表达式匹配过程;
				if v != ')' {
					exp = append(exp, v)
					continue
				}
			}

			// 开头
			if i > 0 && key[i-1] == '\\' { // 转义字符
				out = append(out, v)
			} else if v == ':' { // 命名参数 | 通用表达式
				param = make([]rune, 0)
				startCom = true
			} else if v == '(' { // 正则表达式开始
				startExp = true
				startCom = false

				// 正则表达式前面可能产生参数
				if len(param) > 0 {
					params = append(params, ":"+string(param))
					param = make([]rune, 0)
				}

				paramsNum++
				exp = make([]rune, 0)
				exp = append(exp, '(')
			} else if v == ')' { // 正则表达式结束
				startExp = false
				exp = append(exp, ')')
				out = append(out, exp...)
				param = make([]rune, 0)
			} else if v == '?' { // 匿名参数,
				params = append(params, ":")
			} else {
				out = append(out, v) // 正则表达"(...)", 通用表达式后面的内容
			}
		}

		if len(param) > 0 {
			if paramsNum > 0 {
				out = append(out, []rune(`(.+)`)...)
			}
			params = append(params, ":"+string(param))
		}

		return true, params, string(out)
	}

	return false, nil, ""
}
