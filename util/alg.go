package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"strings"
	"crypto/md5"
	"crypto/sha1"
)

func Hmac256(message string, secret string) string {
	key := []byte(secret)
	h := hmac.New(sha256.New, key)
	h.Write([]byte(message))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func Md5(data []byte) string {
	m := md5.New()
	m.Write(data)
	return string(m.Sum(nil))
}

func Sha256(data []byte) string {
	m := sha256.New()
	m.Write(data)
	return string(m.Sum(nil))
}

func Sha1(data []byte) string {
	m := sha1.New()
	m.Write(data)
	return string(m.Sum(nil))
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
