package main

import (
	_ "net/http"
	"gosource/lib"
)

func init() {
	println("main.init.2")
}

func main() {
	lib.Sum(1, 2, 3)
}

func init() {
	println("main.init.1")
}

/*
编译:
	go build -gcflags "-N -l" -o test

反汇编:
	go tool objdump -s "runtime\.init\b" test // 反汇编runtime.init方法

结论:
	runtime内关联的多个init函数被赋予唯一符号名, 然后再由runtime进行统一调用.

	所有init函数都在同一个goroutine内执行.
	所有init函数结束后才会执行main.main函数
*/