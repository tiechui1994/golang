package utils

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"reflect"
	"strings"
	"time"
	"encoding/hex"
	mrand "math/rand"
	crand "crypto/rand"
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

// HMAC256加密
func ComputeHmac256(message string, secret string) string {
	key := []byte(secret)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

// Struct -> Map
func Struct2Map(value interface{}) (res map[string]interface{}) {
	if _, ok := value.(map[string]interface{}); ok {
		return value.(map[string]interface{})
	}

	res = make(map[string]interface{})
	val := reflect.ValueOf(value)
	if val.Kind() == reflect.Ptr || val.Kind() == reflect.Interface {
		val = val.Elem()
	}

	if val.Kind() != reflect.Struct {
		return nil
	}

	for i := 0; i < val.NumField(); i++ {
		typeField := val.Type().Field(i)
		valueField := val.Field(i)
		res[typeField.Tag.Get("json")] = valueField.Interface()
	}

	return res
}

// 10进制 -> 16进制
func Decimal2Hex(n int64, bit int) string {
	var s string
	if n < 0 {
		n = -n
	}
	if n == 0 {
		return "0x" + strings.Repeat("0", bit)
	}

	hexMap := map[int64]int64{10: 65, 11: 66, 12: 67, 13: 68, 14: 69, 15: 70}
	for q := n; q > 0; q = q / 16 {
		m := q % 16
		if m > 9 && m < 16 {
			m = hexMap[m]
			s = fmt.Sprintf("%v%v", string(m), s)
			continue
		}
		s = fmt.Sprintf("%v%v", m, s)
	}

	if len(s) >= bit {
		return fmt.Sprintf("%v%v", "0x", s)
	}

	return fmt.Sprintf("%v%v", "0x"+strings.Repeat("0", bit-len(s)), s)
}
