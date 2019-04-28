package main

import "reflect"

/*
reflect.Value 结构体解析:

Value是Go值的反射接口.
并非所有方法都适用于所有类型的值. 每种方法的文档中都标明了限制(如果有).
在调用特定于类的方法之前, 使用Kind方法查找值的类型. 调用不适合这种类型的方法会导致运行时出现panic.

零值表示无值. 它的IsValid方法返回false, 它的Kind方法返回Invalid, 它的String方法返回"<invalid Value>", 所有其他方法都会发生panic.
大多数函数和方法永远不会返回无效值. 如果有, 它的文档明确说明条件.

值可以由多个goroutine同时使用, 前提是底层Go值可以同时用于等效的直接操作.

要比较两个值, 请比较Interface方法的结果. 在两个值上使用 == 不会比较它们所代表的基础值.


type flag uintptr

type Value struct {
	// 保存Value表示的值的类型
	typ *rtype

	// 指针值数据, 或者, 如果设置了flagIndir,则指向数据.
	// 设置了flagIndir 或者 typ.pointers()返回true时有效.
	ptr unsafe.Pointer

	// flag 包含有关值的元数据.
	// 最低位是标志位:
	//  -  flagStickyRO: 通过未导出的非嵌入字段获取, 因此只读
	//  -  flagEmbedRO: 通过未导出的嵌入字段获取, 因此是只读的
	//  -  flagIndir: val 包含指向数据的指针
	//  -  flagAddr: v.CanAddr为true(隐含flagIndir)
	//  -  flagMethod: v是方法值.
	//
	// 接下来的五位给出了值的种类.
	//
	// 除了方法值之外, 这会重复typ.Kind().
	// 剩余的 23+ 位给出方法值的方法编号.
	// 如果 flag.kind() != Func, 代码可以假定 flagMethod 未设置.
	// 如果 ifaceIndir(typ), 代码可以假设 flagIndir 已设置.
	flag

	// 方法值表示是当前方法调用, 如某些接收器r的r.Read. typ + val + falg 位描述接收器r,
	// 但标志的Kind位表示Func(方法是函数), flag的顶部位给出r的类型方法表中的方法编号.
}

Call(in []Value) []Value
CallSlice(in []Value) []Value

Type() Type
Kind() Kind
Elem() Value

NumField() int
Field(i int) Value
FieldByIndex(index []int) Value
FieldByName(name string) Value
FieldByNameFunc(match func(string) bool) Value

Index(i int) Value

CanInterface() bool
Interface() interface{}
InterfaceData() [2]uintptr

IsNil() bool
IsValid() bool

MapIndex(key Value) Value
MapKeys() []Value

NumMethod() int
Method(i int) Value
MethodByName(name string) Value

CanAddr() bool
Addr() Value
UnsafeAddr() uintptr
Pointer() uintptr

Cap() int
Len() int

CanSet() bool
Set(x Value)
SetCap(n int)
SetLen(n int)
SetMapIndex(key, val Value)
SetPointer(x unsafe.Pointer)

Slice(i, j int) Value
Slice3(i, j, k int) Value

String() string

Close()

Recv() (x Value, ok bool)
TryRecv() (x Value, ok bool)

Send(x Value)
TrySend(x Value) bool
*/
func main() {
	reflect.ValueOf(10)
}
