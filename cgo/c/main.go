package main

//#include <module.h>
import "C"

func main() {
	C.SayHello(C.CString("Hello, World\n"))
}
