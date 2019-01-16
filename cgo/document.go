package cgo

/***********************************************************************************************************************

相关的命令:
	go tool cgo xxx.go  生成 xxx.go 对应的cgo文件, 注: xxx.go当中必须引入虚拟包 "C"
	go run xxx.go       运行 xxx.go 文件, 必须是main包且包含main()方法

***********************************************************************************************************************/

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

/***********************************************************************************************************************

#cgo 语句:

通过 #cgo 语句可以设置编译阶段和链接阶段的相关参数. 编译阶段的参数主要用于定义相关宏和指定头文件检索路径.
链接阶段的参数主要是指定库文件检索路径和要链接的库文件.

案例:
```
//#cgo CFLAGS: -D PNG_DEBUG=1 -I ./include
//#cgo LDFLAGS: -L /usr/local/lib -l png
//#include <png.h>
import "C"
```

说明: CFLAGS部分, -D定义了宏PNG_DEBUG, 值是1; -I 定义了头文件包含的检索目录.
	 LDFLAGS部分, -L指定了链接时库文件检索目录, -l 指定了链接时需要链接png库.

提示: 因为C/C++遗留的问题, C头文件检索目录可以是相对目录, 但是库文件检索目录则必须是绝对路径.
在库文件的检索目录中可以通过 ${SRCDIR} 变量表示当前包目录的绝对路径:

```
//#cgo LDFLAGS: -L ${SRCDIR}/libs -l foo
```

#cgo 语句主要影响CFLAGS, CPPFLAGS, CXXFLAGS, FFLAGS 和 LDFLAGS 几个编译器环境变量.
LDFLAGS用于设置链接时的参数, 除此之外的几个变量用于改变编译阶段的构建参数(CFLAGS用于针对
C语音代码设置编译参数).

对于在cgo下混合使用C/C++来说, 可能有三种不同的编译选项: 其中CFLAGS对应C语言特有的编译选项,
CXXFLAGS对应是C++特有的编译选项, CPPFLAGS则对应C和C++共有的编译选项. 但是链接阶段, C和
C++的链接选项是通用的.


# cgo 指令还支持条件选择, 当满足某个操作系统或某个CPU架构类型时后面的编译或链接选项的生效.

案例:
```
//#cgo windows CFLAGS: -D X86=1
//#cgo !windows LDFLAGS: -l m
```

说明: windows平台下, 编译前预定义宏X86的值是1
	 非windows平台下, 链接阶段会要求链接math数学库.



条件编译:

build tag 是在Go/cgo环境下的C/C++文件开头的一种特殊注释.

条件编译类似上面通过 #cgo 指令针对不同平台定义的宏, 只有在对应平台的宏被定义之后才会构建对应的代码.
但是, 通过 #cgo 指令定义宏有个限制, 它只能是基于Go语言支持的windows, drawin和linux等已经支持
的操作系统. 如果我们希望定义一个DEBUG标志的宏, #cgo 指令无能为力.

build tag 正是解决 #cgo存在的问题.

案例: 源文件只有在设置debug构建标志才会被构建
```
//+build debug
package cgo

var buildMode = "debug"
```

使用下面的命令构建:
```
go build -tags="debug"
go build -tags="windows debug"
```

当有多个build tag时, 将多个标志通过逻辑操作的规则来组合使用.

案例: 只有在 "linux/386" 或 "darwin/!cgo" 下才能构建
```
//+build linux,386 darwin,!cgo

package cgo
```
说明: "," 表示 AND, " " 表示 OR

***********************************************************************************************************************/

/***********************************************************************************************************************

基本数值类型:

+----------------------+----------------------+---------------------+
|   C TYPE             |    CGO TYPE          |   GO TYPE           |
+----------------------+----------------------+---------------------+
|	char               |	C.char			  |	  byte				|
+----------------------+----------------------+---------------------+
|	signed char	       |    C.schar           |   int8              |
+----------------------+----------------------+---------------------+
|   unsigned char      |    C.uchar         　｜　　uint8   			|
+----------------------+----------------------+---------------------+
|	short			   |    C.short           |   int16             |
+----------------------+----------------------+---------------------+
|	unsigned short     |    C.ushort          |   uint16 			|
+----------------------+----------------------+---------------------+
|	int			       |    C.int             |   int32             |
+----------------------+----------------------+---------------------+
|	unsigned int       |    C.uint            |   uint32 			|
+----------------------+----------------------+---------------------+
|	long			   |    C.long            |   int32             |
+----------------------+----------------------+---------------------+
|	unsigned long      |    C.ulong           |   uint32 			|
+----------------------+----------------------+---------------------+
|	long long int	   |    C.longlong        |   int64             |
+----------------------+----------------------+---------------------+
|unsigned long long int|    C.ulonglong       |   uint64 			|
+----------------------+----------------------+---------------------+
|	float			   |    C.float           |   float32           |
+----------------------+----------------------+---------------------+
|	double             |    C.double          |   float64 			|
+----------------------+----------------------+---------------------+
|	size_t			   |    C.size_t          |   uint              |
+----------------------+----------------------+---------------------+


基本类型对应的C语言类型:

```
typedef signed char GoInt8;
typedef unsigned char GoUint8;
typedef short GoInt16;
typedef unsigned short GoUint16;
typedef int GoInt32;
typedef unsigned int GoUint32;
typedef long long GoInt64;
typedef unsigned long long GoUint64;
typedef GoInt64 GoInt;
typedef GoUint64 GoUint;
typedef float GoFloat32;
typedef double GoFloat64;
```

Go字符串和切片:

在CGO生成的 _cgo_export.h 头文件中还会为Go语言字符串, 切片, 字典, 接口和管道等特有的数据类型
对应的C语言类型:

```
typedef struct {
	const char *p;
	GoInt n;
} GoString;

typedef void *GoMap;
typedef void *GoChan;

typedef struct {
	void *t;
	void *v;
} GoInterface;

typedef struct {
	void *data;
	GoInt len;
	GoInt cap;
} GoSlice;
```


案例:

```
//export helloString
func helloString(s string) {}

//export helloSlice
func helloSlice(s []byte) {}
```

CGO生成的 _cgo_export.h 头文件会包含以下的函数声明:

```
extern void helloString(GoString p0);
extern void helloSlice(GoSlice p0);
```

***********************************************************************************************************************/
