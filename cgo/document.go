package cgo

/***********************************************************************************************************************

注意: 在不同的Go包下引入的 "C" (虚拟包)是不同的. 这就导致不同Go包中引入的虚拟的C包的类型是不同的.

// cgo包
```
package cgo

//#include <stdio.h>
import "C"

type Char C.char

func (p *Char) GoString() string {
    return C.GoString((*C.char)(p))
}

func PrintCString(c *C.char) {
    C.puts(c)
}
```

// main 包, 引入了cgo包
```
package main

//static const char* cs = "hello";
import "C"
import "cgo"

func main() {
    cgo.PrintCString(C.cs)
}
```

上面的例子是无法正常运行的, main包下的虚拟包下的*char(具体就是*main.C.char) 类型和cgo包当中*char(
具体就是*cgo.C.char)类型是不同的. 而且这两者类型在上面的例子当中是无法转换的.

结论: 一个包如果在公开的接口中直接使用了 *C.char 等类似的虚拟包的类型, 其他的Go包是无法直接使用这些类型
的, 除非这个Go包同时了 *C.char 类型的构造函数.

***********************************************************************************************************************/


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

3. 在go当中定义接口并调用(使用C语言)

main.go
```
//#include <stdio.h>
//static void SayHello(const char* s) {
//    puts(s);
//}

import "C"

func main() {
	C.SayHello(C.CString("Hello, World\n"))
}
```

说明: C++/C的关键字说明,参考 keyword.go

************************************************************************************************************************/