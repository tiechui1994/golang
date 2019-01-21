package main

import (
	"fmt"
	"reflect"
	"strings"
)

/*
反射:

顶层设计: reflect.Type
type Type interface {
	// 当内存中分配时, Align()返回此类型值的对齐字节.
	Align() int

	// 当用作结构中的字段时, FieldAlign() 返回此类型值的字节对齐字节.
	FieldAlign() int

	// 方法返回类型方法集中的第i个方法。
	// 如果i不在[0, NumMethod())范围内, 就会发生panic.
	//
	// 对于非接口类型T或*T, 返回的Method的Type和Func字段描述了一个函数, 其第一个参数是接收者.
	// 对于接口类型, 返回的Method的Type字段给出方法体, 没有执行者, Func字段为nil. (目前没有遇到过)
	//
	// 例如: var r Reader = bytes.NewBuffer(nil), r 是一个非接口类型的对象
	//      var w Buffer = bytes.NewBuffer(nil), w 是一个非接口类型的对象
	//
	Method(int) Method


	// MethodByName在类型的方法集中返回具有该名称的方法,并返回指示是否找到该方法的布尔值.
	// 对于非接口类型T或*T, 返回的Method的Type和Func字段描述了一个函数, 其第一个参数是接收者.
	// 对于接口类型, 返回的Method的Type字段给出方法体, 没有执行者, Func字段为nil. (目前没有遇到过)
	MethodByName(string) (Method, bool)

	// 可以导出的方法的数量
	NumMethod() int

	// 返回当前Type的类型名称(包含包名称). 对于未命名的类型, 返回空字符串
	Name() string

	// 返回命名类型的包路径,即唯一标识包的导入路径,
	// 例如"encoding/base64", 如果类型是预先声明的 (string, error) 或未命名的(*T,
	// struct{}, []int), 则包路径将是空字符串.
	PkgPath() string

	// 返回存储给定类型值所需的字节数; 它类似于unsafe.Sizeof。
	Size() uintptr

	// 返回该类型的字符串表示形式. 字符串表示可以使用缩短的包名称(例如, base64而不是"encoding/base64"),
	// 并且不保证在类型之间是唯一的. 要测试类型标识,请直接比较类型
	String() string

	// 返回此类型的特定类型.(int, func, map, slice等)
	Kind() Kind

	// 是否实现了类型u
	Implements(u Type) bool

	// 是否可以将类型的值赋给 类型u.
	AssignableTo(u Type) bool

	// 该类型的值是否可转换为类型u
	ConvertibleTo(u Type) bool

	// 该类型的值是否可进行比较
	Comparable() bool

	// 下面方法仅适用于某些类型,具体取决于Kind.
	// 每种方法允许的方法是:
	//
	//	Int*, Uint*, Float*, Complex*: Bits()
	//	Array: Elem(), Len()
	//	Chan: ChanDir(), Elem()
	//	Func: In(), NumIn(), Out(), NumOut(), IsVariadic().
	//	Map: Key, Elem()
	//	Ptr: Elem()
	//	Slice: Elem()
	//	Struct: Field(), FieldByIndex(), FieldByName(), FieldByNameFunc(), NumField()
	//

	// 以bit为单位返回类型的大小. 如果类型的Kind不是Int,Uint,Float或Complex类型之一,则会panic
	Bits() int

	// 返回通道类型的方向. 如果类型的种类不是Chan，它会发生panic
	ChanDir() ChanDir

	// 例子说明:
	// func(x int, y ...float64)
	//
	//	t.NumIn() == 2
	//	t.In(0) is the reflect.Type for "int"
	//	t.In(1) is the reflect.Type for "[]float64"
	//	t.IsVariadic() == true
	//
	// 如果类型不是Func, panic
	IsVariadic() bool

	// Elem返回一个类型的元素类型. 如果类型的Kind不是Array, Chan,Map, Ptr或Slice, 它会发生panic
	Elem() Type

	// Field返回Struct类型的第i个字段.
	// 如果类型的Kind不是Struct, 则会发生panic.
	// 如果我不在[0, NumField()) 范围内,也会发生panic
	Field(i int) StructField

	// FieldByIndex返回与索引序列对应的嵌套字段. 它相当于为每个索引i连续调用Field.
	// 如果类型的Kind不是Struct, 则会发生panic.
	FieldByIndex(index []int) StructField

	// 返回具有给定名称的struct字段和一个是否找到该字段的布尔值.
	FieldByName(name string) (StructField, bool)


	// 返回结构字段, 其名称满足匹配函数, 以及一个是否找到该字段的布尔值.
	//
	// FieldByNameFunc的函数先匹配结构本身中的字段, 然后匹配任何匿名结构中的字段, 按广度优先顺序,
	// 在包含满足匹配函数的一个或多个字段的最浅嵌套深度处停止.
	// 如果该深度的多个字段满足匹配函数, 则它们相互抵消, 并且FieldByNameFunc不返回该匹配项
	// 此行为反映了Go在包含匿名字段的结构中对名称查找的处理.
	FieldByNameFunc(match func(string) bool) (StructField, bool)


	// 返回map类型当中key的类型
	// 如果类型的Kind不是Map, panic
	Key() Type

	// 返回Array类型的长度
	// 如果类型的Kind不是Array, panic
	Len() int

	// 返回Struct类型的字段总数
	// 如果类型的Kind不是Struct, panic
	NumField() int

	// 返回Func类型输入参数的个数
	// 如果类型的Kind不是Func, panic
	NumIn() int

	// 返回Func类型的输出参数个数
	// 如果类型的Kind不是Func, panic
	NumOut() int

	// 返回Func类型的第i个输出参数的类型
	// 如果类型的Kind不是Func, panic
	// 如果i不在[0, NumOut())之间, panic
	Out(i int) Type

	// 返回Func类型的第i个输入参数的类型
	// 如果类型的Kind不是Func, panic
	// 如果i不在[0, NumIn())之间, panic
	In(i int) Type
}


*/
type Inter interface {
	Say(str string)
}

type Interface struct {
	A string
	B int
}

func (i *Interface) Say(str string) {
	fmt.Println(str)
}

type Struct struct {
}

func (s *Struct) SayWord(str string) {
	fmt.Println(str)
}

func SayHello(str string) {
	fmt.Println(str)
}

func main() {
	var i Inter = &Interface{}
	it := reflect.TypeOf(i)
	fmt.Printf("Align:%v \n", it.Align())
	fmt.Printf("FieldAlign:%v \n", it.FieldAlign())
	fmt.Println(strings.Repeat("-", 10))
	fmt.Printf("Interface Type NumMethod:%v \n", it.NumMethod())
	fmt.Printf("Interface Type Method:%+v \n", it.Method(0))
	fmt.Println(it.Elem().Name())

	fmt.Println(strings.Repeat("==", 15))

	var s = &Struct{}
	st := reflect.TypeOf(s)
	fmt.Printf("Struct Type NumMethod:%v \n", st.NumMethod())
	fmt.Printf("Struct Type Method:%+v \n", st.Method(0))
	fmt.Println(st.Elem().Name())

	fmt.Println(strings.Repeat("==", 15))
}
