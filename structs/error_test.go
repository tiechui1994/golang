package structs

import (
	"testing"
	"fmt"
	"unsafe"
	"reflect"
	"time"
	"math/rand"
)

/*
匿名函数中记录的是循环变量的内存地址, 而不是循环变量某一刻的值
*/

func GetRandomString(length int) string {
	var (
		result []byte
		bytes  = []byte("0123456789abcdefghijklmnopqrstuvwxyz")
	)

	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	for i := 0; i < length; i++ {
		result = append(result, bytes[r.Intn(len(bytes))])
	}
	return string(result)
}

func TestArrayRange(t *testing.T) {
	var msg []func()
	var array = []string{"1", "2", "3"}
	for _, v := range array {
		// ele := v  创建变量ele,存储v的值
		msg = append(msg, func() {
			fmt.Println(v)
		})
	}

	for _, v := range msg {
		v()
	}
}

/*
slice是一块完整的内存地址. &取地址可以索引到具体的地址
*/

func TestArray(t *testing.T) {
	var array = []int{1, 2}
	fmt.Println(unsafe.Pointer(reflect.ValueOf(array).Pointer()), &array[0], &array[1], len(array), cap(array))
	array = append(array, 10, 30, 8, 10)
	fmt.Println(unsafe.Pointer(reflect.ValueOf(array).Pointer()), &array[0], &array[1], len(array), cap(array))

	var mp = map[string]string{
		GetRandomString(5): GetRandomString(8),
		GetRandomString(5): GetRandomString(8),
	}
	fmt.Println(unsafe.Pointer(reflect.ValueOf(mp).Pointer()))
	for i := 0; i < 2000; i++ {
		mp[GetRandomString(5)] = GetRandomString(8)
	}
	fmt.Println(unsafe.Pointer(reflect.ValueOf(mp).Pointer()))
}

/*
解密: Slice, String, Map的底层数据结构
*/
func TestData(t *testing.T) {
	type Slice struct {
		array uintptr // 一个数组元素类型的指针, 即数组的首地址
		len   int
		cap   int
	}

	var slice = make([]int32, 5, 10)
	pSlice := (*Slice)(unsafe.Pointer(&slice))
	fmt.Printf("%+v\n", pSlice)
	p1 := (*int32)(unsafe.Pointer(pSlice.array))
	fmt.Println(int32(*p1))

	type String struct {
		str uintptr // 一个byte指针, 即[]byte的首地址
		len int
	}
	var str = "1"
	pString := (*String)(unsafe.Pointer(&str))
	fmt.Printf("%+v\n", pString)
	p2 := (*byte)(unsafe.Pointer(pString.str))
	fmt.Println(string(*p2))

	type Map struct {
		count      int
		flags      uint32
		hash0      uint32
		B          uint8
		keysize    uint8
		valuesize  uint8
		bucketsize uint16
		buckets    uintptr
		oldbuckets uintptr
		nevacuate  uintptr
	}
	var mp = make(map[string]int32, 5)
	mp["hello"] = 123
	pMap := (*Map)(unsafe.Pointer(&mp))
	fmt.Printf("%+v", *pMap)
}
