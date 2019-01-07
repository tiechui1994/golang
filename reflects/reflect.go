package main

import (
	"reflect"
	"fmt"
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
	// 对于接口类型, 返回的Method的Type字段给出方法体, 没有执行者, Func字段为nil.
	//
	// 接口类型: 被反射的对象的type是接口类型, value是结构体
	// 非接口类型: 被反射的对象的type是结构体, value是结构体
	// 例如: var r Reader = bytes.NewBuffer(nil), r 是一个接口类型的对象
	//      var w Buffer = bytes.NewBuffer(nil), w 是一个非接口类型的对象
	//
	Method(int) Method


	// MethodByName在类型的方法集中返回具有该名称的方法,并返回指示是否找到该方法的布尔值.
	MethodByName(string) (Method, bool)

	// 可以导出的方法的数量
	NumMethod() int

	// Name returns the type's name within its package.
	// It returns an empty string for unnamed types.
	Name() string

	// PkgPath returns a named type's package path, that is, the import path
	// that uniquely identifies the package, such as "encoding/base64".
	// If the type was predeclared (string, error) or unnamed (*T, struct{}, []int),
	// the package path will be the empty string.
	PkgPath() string

	// Size returns the number of bytes needed to store
	// a value of the given type; it is analogous to unsafe.Sizeof.
	Size() uintptr

	// String returns a string representation of the type.
	// The string representation may use shortened package names
	// (e.g., base64 instead of "encoding/base64") and is not
	// guaranteed to be unique among types. To test for type identity,
	// compare the Types directly.
	String() string

	// Kind returns the specific kind of this type.
	Kind() Kind

	// Implements reports whether the type implements the interface type u.
	Implements(u Type) bool

	// AssignableTo reports whether a value of the type is assignable to type u.
	AssignableTo(u Type) bool

	// ConvertibleTo reports whether a value of the type is convertible to type u.
	ConvertibleTo(u Type) bool

	// Comparable reports whether values of this type are comparable.
	Comparable() bool

	// Methods applicable only to some types, depending on Kind.
	// The methods allowed for each kind are:
	//
	//	Int*, Uint*, Float*, Complex*: Bits
	//	Array: Elem, Len
	//	Chan: ChanDir, Elem
	//	Func: In, NumIn, Out, NumOut, IsVariadic.
	//	Map: Key, Elem
	//	Ptr: Elem
	//	Slice: Elem
	//	Struct: Field, FieldByIndex, FieldByName, FieldByNameFunc, NumField

	// Bits returns the size of the type in bits.
	// It panics if the type's Kind is not one of the
	// sized or unsized Int, Uint, Float, or Complex kinds.
	Bits() int

	// ChanDir returns a channel type's direction.
	// It panics if the type's Kind is not Chan.
	ChanDir() ChanDir

	// IsVariadic reports whether a function type's final input parameter
	// is a "..." parameter. If so, t.In(t.NumIn() - 1) returns the parameter's
	// implicit actual type []T.
	//
	// For concreteness, if t represents func(x int, y ... float64), then
	//
	//	t.NumIn() == 2
	//	t.In(0) is the reflect.Type for "int"
	//	t.In(1) is the reflect.Type for "[]float64"
	//	t.IsVariadic() == true
	//
	// IsVariadic panics if the type's Kind is not Func.
	IsVariadic() bool

	// Elem returns a type's element type.
	// It panics if the type's Kind is not Array, Chan, Map, Ptr, or Slice.
	Elem() Type

	// Field returns a struct type's i'th field.
	// It panics if the type's Kind is not Struct.
	// It panics if i is not in the range [0, NumField()).
	Field(i int) StructField

	// FieldByIndex returns the nested field corresponding
	// to the index sequence. It is equivalent to calling Field
	// successively for each index i.
	// It panics if the type's Kind is not Struct.
	FieldByIndex(index []int) StructField

	// FieldByName returns the struct field with the given name
	// and a boolean indicating if the field was found.
	FieldByName(name string) (StructField, bool)

	// FieldByNameFunc returns the struct field with a name
	// that satisfies the match function and a boolean indicating if
	// the field was found.
	//
	// FieldByNameFunc considers the fields in the struct itself
	// and then the fields in any anonymous structs, in breadth first order,
	// stopping at the shallowest nesting depth containing one or more
	// fields satisfying the match function. If multiple fields at that depth
	// satisfy the match function, they cancel each other
	// and FieldByNameFunc returns no match.
	// This behavior mirrors Go's handling of name lookup in
	// structs containing anonymous fields.
	FieldByNameFunc(match func(string) bool) (StructField, bool)

	// In returns the type of a function type's i'th input parameter.
	// It panics if the type's Kind is not Func.
	// It panics if i is not in the range [0, NumIn()).
	In(i int) Type

	// Key returns a map type's key type.
	// It panics if the type's Kind is not Map.
	Key() Type

	// Len returns an array type's length.
	// It panics if the type's Kind is not Array.
	Len() int

	// NumField returns a struct type's field count.
	// It panics if the type's Kind is not Struct.
	NumField() int

	// NumIn returns a function type's input parameter count.
	// It panics if the type's Kind is not Func.
	NumIn() int

	// NumOut returns a function type's output parameter count.
	// It panics if the type's Kind is not Func.
	NumOut() int

	// Out returns the type of a function type's i'th output parameter.
	// It panics if the type's Kind is not Func.
	// It panics if i is not in the range [0, NumOut()).
	Out(i int) Type
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

func SayWord(str string) {
	fmt.Println(str)
}

func main() {
	var i Inter = &Interface{}
	it := reflect.TypeOf(i)
	fmt.Printf("Align:%v \n", it.Align())
	fmt.Printf("FieldAlign:%v \n", it.FieldAlign())

	fmt.Printf("Interface Type NumMethod:%v \n", it.NumMethod())
	fmt.Printf("Interface Type Method:%+v \n", it.Method(0))

	fmt.Println(strings.Repeat("==", 10))

	var s = &Struct{}
	st := reflect.TypeOf(s)
	fmt.Printf("Struct Type NumMethod:%v \n", st.NumMethod())
	fmt.Printf("Struct Type Method:%+v \n", st.Method(0))

	fmt.Println(strings.Repeat("==", 10))

	var m = SayWord
	mt := reflect.TypeOf(m)
	fmt.Printf("Method Type NumMethod:%v \n", mt.NumMethod())
}
