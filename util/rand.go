package util

import (
	"time"
	"encoding/hex"

	crand "crypto/rand"
	mrand "math/rand"
)

// 随机字符串
func RandomString(length int) string {
	bytes := []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	result := make([]byte, 0)
	r := mrand.New(mrand.NewSource(time.Now().UnixNano())) // 产生随机数实例
	for i := 0; i < length; i++ {
		result = append(result, bytes[r.Intn(len(bytes))]) // 获取随机
	}
	return string(result)
}

// 随机字符串
func RandomStr(length int) string {
	data := make([]byte, length)
	n, _ := crand.Read(data)
	if n == length {
		return hex.EncodeToString(data)
	}
	return RandomStr(length)
}

// 随机整数
func RandInt(scope int) int {
	r := mrand.New(mrand.NewSource(time.Now().UnixNano())) // 产生随机数实例
	return r.Intn(scope)
}
