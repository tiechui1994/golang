package main

import (
	"unsafe"
	"fmt"
)

/**
内存布局:
unsafe顶层设计:
	type ArbitraryType int

	// 指针表示指向任意类型的指针. 类型指针有四种特殊操作,不适用于其他类型:
	//  - 任何类型的指针值都可以转换为Pointer.
	//  - Pointer可以转换为任何类型的指针值.
	//  - uintptr可以转换为Pointer.
	//  - Pointer可以转换为uintptr.

	// Pointer因此允许程序打破类型系统并读写任意内存. 应该特别小心使用.
	//
	// 以下涉及Pointer的模式是合法的.
	// (1) 将*T1转换为Pointer到*T2.
	//
	// 假设T2不大于T1并且两者共享等效的内存布局, 则此转换允许将一种类型的数据重新解释为另一种类型的数据.
	// 一个例子是math.Float64bits的实现:
	//
	// func Float64bits(f float64) uint64 {
	//     return *(*uint64)(unsafe.Pointer(&f))
	// }
	//
	// (2)将Pointer转换为uintptr(但不返回Pointer).
	//
	// 将Pointer转换为uintptr会产生指向的值的内存地址(作为整数). 这种uintptr的通常用途是打印它.
	// 将uintptr转换回Pointer一般无效.
	//
	// uintptr是整数, 而不是引用. 将Pointer转换为uintptr会创建一个没有语义的整数值.
	// 即uintptr保存某个对象的地址. 如果当此对象移动,垃圾收集器不会更新该uintptr的值, uintptr也不会保证该对象不被回收.
	// 其余模式枚举了从uintptr到Pointer的唯一有效的转换.
	//
	// (3)使用算术将Pointer转换为uintptr并返回.
	//
	// 如果p指向一个已分配的对象, 则可以通过转换为uintptr, 添加偏移量(Offset)来转换回指针.
	// p = unsafe.Pointer(uintptr(p) + offset)
	//
	// 此模式最常见的用途是访问结构中的字段或数组的元素:
	//
	// // 等效于 f:= unsafe.Pointer(&s.f)
	//          f:= unsafe.Pointer(uintptr(unsafe.Pointer(&s)) + unsafe.Offsetof(s.f))
	//
	// // 等效于 e:= unsafe.Pointer(&x[i])
	//          e:= unsafe.Pointer(uintptr(unsafe.Pointer(&x[0])) + i * unsafe.Sizeof(x[0]))
	//
	// 以这种方式添加和减去指针的偏移量都是有效的.使用&^来循环指针也很有效,通常用于对. 在所有情况下,结果必须继续指向原始分配的对象.
	//
	// 与C不同,将指针推进到原始分配的末尾是无效的:
	//
	// // invalid: end points outside allocated space.
	// var s string
	// end := unsafe.Pointer(uintptr(unsafe.Pointer(&s))+ unsafe.Sizeof(s))
	//
	// // invalid: end points outside allocated space.
	// b := make([]byte, n)
	// end := unsafe.Pointer(uintptr(unsafe.Pointer(&b[0]))+ uintptr(n))
	//
	// 注意, 两个转换必须出现在同一个表达式中, 它们之间只有使用算术:
	//
	// // invalid: 在转换回指针之前,uintptr不能存储在变量中.
	// u := uintptr(p)
	// p = unsafe.Pointer(u + offset)
	//
	// (4) 在调用syscall.Syscall时将Pointer转换为uintptr.
	//
	// syscall包中的Syscall函数将它们的uintptr参数直接传递给操作系统, 操作系统可能会根据调用的详细信息将其中的
	// 一些重新解释(reinterpret)为指针. 也就是说,系统调用实现隐式地将某些参数从uintptr转换回指针.
	//
	// 如果必须将指针参数转换为uintptr以用作参数, 则该转换必须出现在调用表达式本身中:
	//
	// syscall.Syscall(SYS_READ,uintptr(fd),uintptr(unsafe.Pointer(p)),uintptr(n))
	//
	// 编译器通过排列引用的已分配对象(如果有)保留并且在调用完成之前不会移动来处理在程序集中实现的函数调用
	// 的参数列表中转换为uintptr的指针,即使从类型中也是如此在呼叫期间,似乎不再需要该对象.
	//
	// 为使编译器识别此模式,转换必须出现在参数列表中:
	//
	// // invalid: 系统调用期间, 在隐式转换返回回指针之前, uintptr不能存储在变量中
	// u:= uintptr(unsafe.Pointer(p))
	// syscall.Syscall(SYS_READ,uintptr(fd),u,uintptr(n))
	//
	// (5) 将reflect.Value.Pointer或reflect.Value.UnsafeAddr的结果从uintptr转换为Pointer.
	// 包反射名为Pointer和UnsafeAddr的值方法返回类型uintptr而不是unsafe.Pointer, 以防止调用者在
	// 不先导入"unsafe"的情况下将结果更改为任意类型. 但是,这意味着结果很脆弱,必须在相同表达式中调用后立即转换为Pointer.
	//
	// p:=(*int)(unsafe.Pointer(reflect.ValueOf(new(int)).Pointer()))
	//
	// 与上面的情况一样,在转换之前存储结果是无效的:
	//
	// // invalid:在转换回指针之前, uintptr不能存储在变量中
	// u:= reflect.ValueOf(new(int)).Pointer()
	// p:= (*int)(unsafe.Pointer(u))
	//
	// (6) 将一个reflect.SliceHeader或reflect.StringHeader数据字段转换为Pointer或从Pointer转换.
	//
	// 与前一种情况一样, 反射数据结构SliceHeader和StringHeader将字段Data声明为uintptr,
	// 以防止调用者在不先导入"unsafe"的情况下将结果更改为任意类型. 但这意味着SliceHeader和StringHeader仅在解析实际
	// 切片或字符串值的内容时有效.
	//
	// var s string
	// hdr := (*reflect.StringHeader)(unsafe.Pointer(&s)) // 案例1
	// hdr.Data = uintptr(unsafe.Pointer(p))	// 案例6(本例)
	// hdr.Len = n
	//
	// 在这种用法中, hdr.Data实际上是一种引用切片头中的底层指针的替代方法,而不是uintptr变量本身.
	//
	// 通常,reflect.SliceHeader和reflect.StringHeader只能用作 *reflect.SliceHeader和*reflect.StringHeader,
	// 指向实际的切片或字符串,而不是普通的结构. 一个程序不应声明或分配这些结构类型的变量.
	//
	// // invalid: a directly-declared header will not hold Data as a reference.
	// var hdr reflect.StringHeader
	// hdr.Data = uintptr(unsafe.Pointer(p))
	// hdr.Len = n
	// s:= *(*string)(unsafe.Pointer(&hdr)) // p可能已经丢失了
	type Pointer *ArbitraryType

unsafe包的方法说明:
	unsafe.Pointer() 将一个指针转换成

	unsafe.Alignof() 获取变量的对齐值, 除了int, uintptr依赖于CPU位数的类型之外,基本类型的对齐值都是固定的.结构体的对齐值取
成员对齐值的最大值.

	特别说明的几个:
		float32 	8
		complex 	8
		chan		8
		slice		8
		map			8
		string		8
		struct{} 	0

	unsafe.Sizeof()  获取变量所占的字节数
	特别说明(零值):
		string		16
		slice		24
		map			8
		chan  		8
		struct{} 	1

	unsafe.Offsetof()  获取结构体成员的偏移字节数, 即:结构体成员的开始指针位置.

	以0x0作为基准内存的地址.
	对齐必须满足的条件: p % align == 0, p为结构体成员的开始指针. align作为结构体成员的对齐字节数.
	结论: p + size 作为结构体成员的结束指针

	结构体size = 结构体最后一个成员的开始指针 + 结构体的align

	unsafe.Pointer() 指针转换的中介者, 表示任意指针. 传入的参数也是一个指针.
*/

func main() {
	i := new(int)
	x := unsafe.Pointer(i)
	fmt.Println(unsafe.Alignof(x))
}
