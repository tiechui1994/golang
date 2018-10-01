package main

import (
	"fmt"
	"encoding/gob"
	"bytes"
	"encoding/base64"
	"crypto/cipher"
	crand "crypto/rand"
	"errors"
	"container/list"
	"crypto/aes"
	"os"
	"time"
	"crypto/hmac"
	"crypto/sha1"
	"sync"
	"runtime"
)

func generateRandomKey(length int) (data []byte) {
	data = make([]byte, length)
	n, err := crand.Read(data)
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

func cookieExample() {
	var (
		name     = "cookie"
		hashKey  = "hash"
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
	bytes, _ = Encrypt(block, bytes)
	// Base64编码
	bytes = Encode(bytes)
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

func List() {
	var (
		store = make(map[string]*list.Element)
		l     = list.List{}
	)

	ele := &struct {
		Name string
	}{Name: "张三"}

	element := l.PushFront(ele)
	store["1"] = element

	fmt.Printf("%+v", store)
}

type threadSafeSet struct {
	sync.RWMutex
	s []interface{}
}

func (set *threadSafeSet) Iter() <-chan interface{} {
	ch := make(chan interface{}) // 同步处理, 处理的速度有限
	//ch := make(chan interface{}, len(set.s)) // 异步处理, 处理的速度得到提升
	go func() {
		set.RLock()

		for elem, value := range set.s {
			ch <- value
			fmt.Println("Iter:", elem, value)
		}

		close(ch)
		set.RUnlock()

	}()
	return ch
}

func main() {
	runtime.GOMAXPROCS(1)
	wg := sync.WaitGroup{}
	wg.Add(10)
	for i := 0; i < 10; i++ {
		go func() {
			fmt.Println("i: ", i)
			wg.Done()
		}()
	}
	for i := 0; i < 10; i++ {
		go func(i int) {
			fmt.Println("i: ", i)
			wg.Done()
		}(i)
	}
	wg.Wait()
}
