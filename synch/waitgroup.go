package synch

import (
	"unsafe"
)

/*
unsafe包的方法说明:
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
	特别说明:
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
type WaitGroup struct {
	state1 [12]byte
	sema   uint32
}

/*
unsafe.Pointer其实就是类似C的void *, 在golang中是用于各种指针相互转换的桥梁.
uintptr是golang的内置类型,是能存储指针的整型,uintptr的底层类型是int,它和unsafe.Pointer可相互转换.

uintptr vs unsafe.Pointer的区别就是:
	unsafe.Pointer只是单纯的通用指针类型,用于转换不同类型指针,它不可以参与指针运算;
	uintptr是用于指针运算的,GC不把uintptr当指针,也就是说uintptr无法持有对象,uintptr类型的目标会被回收.
*/

// state()函数可以获取到wg.state1数组中元素组成的二进制对应的十进制的值
func (wg *WaitGroup) state() *uint64 {
	if uintptr(unsafe.Pointer(&wg.state1))%8 == 0 {
		return (*uint64)(unsafe.Pointer(&wg.state1))
	} else {
		return (*uint64)(unsafe.Pointer(&wg.state1[4]))
	}
}
