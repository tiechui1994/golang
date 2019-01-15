package cgo

/***********************************************************************************************************************

1. 在C中定义接口

函数的定义: module.h

函数的实现: module.c (使用C实现)
		  module.cpp (使用C++实现)
          module.go (使用go实现)

module.h
```
extern void SayHello(const char* str);
```

module.go
```
package main

import "C"
import "fmt"

//export SayHello
func SayHello(s *C.char)  {
	fmt.Println(C.GoString(s))
}
```

main.go (测试)
```
//#include <module.h>
import "C"

func main() {
	C.SayHello(C.CString("Hello, World\n"))
}
```

注: 因为多个main依赖, 因此使用 `go run  *.go` 进行执行

2. 在go当中定义接口

函数的定义: main.go

函数的实现: module.go

main.go
```
//extern void SayHello(const char* s);
import "C"

func main() {
	C.SayHello(C.CString("Hello, World\n"))
}
```

module.go
```
package main

import "C"
import "fmt"

//export SayHello
func SayHello(s *C.char)  {
	fmt.Println(C.GoString(s))
}
```

************************************************************************************************************************/