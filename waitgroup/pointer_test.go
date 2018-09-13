package waitgroup

import (
	"unsafe"
	"testing"
	"fmt"
	"crypto/rand"
)

func TestByteArray(t *testing.T) {
	var state = []byte{1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1}
	fmt.Println(uintptr(unsafe.Pointer(&state)) == 842350502080)
	if uintptr(unsafe.Pointer(&state))%8 == 0 {
		fmt.Println((*uint64)(unsafe.Pointer(&state)))
	}
	for i := 0; i < 100; i++ {
		state = make([]byte, 12)
		rand.Read(state)
		fmt.Println(uintptr(unsafe.Pointer(&state)))
	}

	for i := 0; i < 20000; i++ {
		state = make([]byte, 12)
		rand.Read(state)
		fmt.Println(uintptr(unsafe.Pointer(&state)))
	}

}
