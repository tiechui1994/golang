package main

import "reflect"

/*
reflect.Value 结构体解析:

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

	// A method value represents a curried method invocation
	// like r.Read for some receiver r. The typ+val+flag bits describe
	// the receiver r, but the flag's Kind bits say Func (methods are
	// functions), and the top bits of the flag give the method number
	// in r's type's method table.
	// 方法值表示一个 curried 方法调用, 如某些接收器r的r.Read. typ + val + falg 位描述接收器r,
	// 但标志的Kind位表示Func(方法是函数), flag的顶部位给出r的类型方法表中的方法号.
}
*/
func main() {
	reflect.ValueOf(10)
}
