package main

import (
	"strings"
	"regexp"
	"fmt"
)

func splitSegment(key string) (bool, []string, string) {
	if strings.HasPrefix(key, "*") {
		if key == "*.*" {
			return true, []string{".", ":path", ":ext"}, ""
		}
		return true, []string{":splat"}, ""
	}
	if strings.ContainsAny(key, ":") {
		var paramsNum int
		var out []rune
		var start bool
		var startexp bool
		var param []rune
		var expt []rune
		var skipnum int
		params := []string{}
		reg := regexp.MustCompile(`[a-zA-Z0-9_]+`)
		for i, v := range key {
			if skipnum > 0 {
				skipnum--
				continue
			}
			if start {
				//:id:int and :name:string
				if v == ':' {
					if len(key) >= i+4 {
						if key[i+1:i+4] == "int" {
							out = append(out, []rune("([0-9]+)")...)
							params = append(params, ":"+string(param))
							start = false
							startexp = false
							skipnum = 3
							param = make([]rune, 0)
							paramsNum++
							continue
						}
					}
					if len(key) >= i+7 {
						if key[i+1:i+7] == "string" {
							out = append(out, []rune(`([\w]+)`)...)
							params = append(params, ":"+string(param))
							paramsNum++
							start = false
							startexp = false
							skipnum = 6
							param = make([]rune, 0)
							continue
						}
					}
				}
				// params only support a-zA-Z0-9
				if reg.MatchString(string(v)) {
					param = append(param, v)
					continue
				}
				if v != '(' {
					out = append(out, []rune(`(.+)`)...)
					params = append(params, ":"+string(param))
					param = make([]rune, 0)
					paramsNum++
					start = false
					startexp = false
				}
			}
			if startexp {
				if v != ')' {
					expt = append(expt, v)
					continue
				}
			}
			// Escape Sequence '\'
			if i > 0 && key[i-1] == '\\' {
				out = append(out, v)
			} else if v == ':' {
				param = make([]rune, 0)
				start = true
			} else if v == '(' {
				startexp = true
				start = false
				if len(param) > 0 {
					params = append(params, ":"+string(param))
					param = make([]rune, 0)
				}
				paramsNum++
				expt = make([]rune, 0)
				expt = append(expt, '(')
			} else if v == ')' {
				startexp = false
				expt = append(expt, ')')
				out = append(out, expt...)
				param = make([]rune, 0)
			} else if v == '?' {
				params = append(params, ":")
			} else {
				out = append(out, v)
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

func main() {
	fmt.Println(splitSegment(":id\t"))
}