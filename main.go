package main

import (
	"github.com/astaxie/beego/session"
	"fmt"
	"encoding/gob"
	"bytes"
	"encoding/base64"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"crypto/aes"
	"os"
	"time"
	"crypto/hmac"
	"crypto/sha1"
)

// Session 测试
func init() {
	managerConfig := session.ManagerConfig{
		CookieName:      "gosessionid",
		EnableSetCookie: true,
		Gclifetime:      3600,
		Maxlifetime:     3600,
		Secure:          false,
		CookieLifeTime:  3600,
		ProviderConfig:  "",
	}
	globalSessions, _ := session.NewManager("memory", &managerConfig)
	fmt.Printf("%+v\n", globalSessions)
	go globalSessions.GC()
}

func generateRandomKey(length int) (data []byte) {
	data = make([]byte, length)
	n, err := rand.Read(data)
	if n != length || err != nil {
		return nil
	}

	return data
}

func EncodeGob(obj map[interface{}]interface{}) ([]byte, error) {
	for _, v := range obj {
		gob.Register(v)
	}
	buf := bytes.NewBuffer(nil)
	enc := gob.NewEncoder(buf)
	err := enc.Encode(obj)
	if err != nil {
		return []byte(""), err
	}
	return buf.Bytes(), nil
}

func DecodeGob(encoded []byte) (map[interface{}]interface{}, error) {
	buf := bytes.NewBuffer(encoded)
	dec := gob.NewDecoder(buf)
	var out map[interface{}]interface{}
	err := dec.Decode(&out)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func Encode(value []byte) []byte {
	encoded := make([]byte, base64.URLEncoding.EncodedLen(len(value)))
	base64.URLEncoding.Encode(encoded, value)
	return encoded
}

func Decode(value []byte) ([]byte, error) {
	decoded := make([]byte, base64.URLEncoding.DecodedLen(len(value)))
	b, err := base64.URLEncoding.Decode(decoded, value)
	if err != nil {
		return nil, err
	}
	return decoded[:b], nil
}

func Encrypt(block cipher.Block, value []byte) ([]byte, error) {
	iv := generateRandomKey(block.BlockSize())
	if iv == nil {
		return nil, errors.New("encrypt: failed to generate random iv")
	}
	// Encrypt it.
	stream := cipher.NewCTR(block, iv)
	stream.XORKeyStream(value, value)
	// Return iv + ciphertext.
	return append(iv, value...), nil
}
func main() {
	var (
		name = "cookie"
		hashKey = "hash"
		blockKey = generateRandomKey(16)
	)
	block, err := aes.NewCipher([]byte(blockKey))
	if err != nil {
		os.Exit(-1)
	}

	origin := map[interface{}]interface{}{
		"A": 123,
		"B": "Java",
		"C": 2.15,
		"D": "Y",
	}

	// gob流
	bytes, _ := EncodeGob(origin)
	// 加密
	bytes, _ =  Encrypt(block, bytes)
	// Base64编码
	bytes  = Encode(bytes)
	// 唯一MAC
	bytes = []byte(fmt.Sprintf("%s|%d|%s|", name, time.Now().UTC().Unix(), bytes))
	h := hmac.New(sha1.New, []byte(hashKey))
	h.Write(bytes)
	sig := h.Sum(nil)
	bytes = append(bytes, sig...)[len(name)+1:]
	// Base64编码
	bytes = Encode(bytes)
	fmt.Println(string(bytes))
}
