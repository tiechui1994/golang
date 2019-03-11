package utils

import (
	"bytes"
	"strconv"
	"runtime"
)

// 通过stack信息获取goroutine id
func GetGID() uint64 {
	data := make([]byte, 64)
	length := runtime.Stack(data, false)                         // 获取stack信息
	data = bytes.TrimPrefix(data[:length], []byte("goroutine ")) // 去除前缀
	id := data[:bytes.IndexByte(data, ' ')]                      // 获取id后面的空隔位置
	n, _ := strconv.ParseUint(string(id), 10, 64)                // 转换
	return n
}
