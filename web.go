package main

import (
	"strings"
	"regexp"
	"fmt"
	"reflect"
	"path"
)

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

func TestSplitSegment() {
	items := map[string]struct {
		isReg  bool
		params []string
		regStr string
	}{
		"admin":                      {false, nil, ""},
		"*":                          {true, []string{":splat"}, ""},
		"*.*":                        {true, []string{".", ":path", ":ext"}, ""},
		":id":                        {true, []string{":id"}, ""},
		"?:id":                       {true, []string{":", ":id"}, ""},
		":id:int":                    {true, []string{":id"}, "([0-9]+)"},
		":name:string":               {true, []string{":name"}, `([\w]+)`},
		":id([0-9]+)":                {true, []string{":id"}, `([0-9]+)`},
		":id([0-9]+)_:name":          {true, []string{":id", ":name"}, `([0-9]+)_(.+)`},
		":id(.+)_cms.html":           {true, []string{":id"}, `(.+)_cms.html`},
		"cms_:id(.+)_:page(.+).html": {true, []string{":id", ":page"}, `cms_(.+)_(.+).html`},
		`:app(a|b|c)`:                {true, []string{":app"}, `(a|b|c)`},
		`:app\((a|b|c)\)`:            {true, []string{":app"}, `(.+)\((a|b|c)\)`},
	}

	for pattern, v := range items {
		b, w, r := splitSegment(pattern)
		if b != v.isReg || r != v.regStr || strings.Join(w, ",") != strings.Join(v.params, ",") {
			fmt.Println("error")
		}
	}
}

type SiteController struct {
	Name string
}

func main() {
	site := SiteController{}

	siteType := reflect.TypeOf(site)

	fmt.Println(path.Join(siteType.PkgPath(), siteType.Name()))
}
