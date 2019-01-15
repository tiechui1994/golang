package main

import "C"
import "fmt"

//export SayHello
func SayHello(s *C.char) {
	fmt.Println(C.GoString(s))
}
