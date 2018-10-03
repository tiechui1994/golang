package synch

import (
	"unsafe"
	"sync/atomic"
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
type WaitGroup struct {
	// 8+4策略
	// 在64位机器上, 高8*8位作为计数器, 低4*8位作为goroutine的等待计数
	// 在32位机器上, 中4*8位作为计数器, 低4*8位作为goroutine的等待计数. (高4位空置)
	// 在64位机器上, slice的对齐为8, 在32位机器上slice对齐应该为4
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
		return (*uint64)(unsafe.Pointer(&wg.state1)) // 获取高8位开始指针
	} else {
		return (*uint64)(unsafe.Pointer(&wg.state1[4])) // 获取中4位开始指针
	}
}

func (wg *WaitGroup) Add(delta int) {
	// 获取的是指针的位置
	statep := wg.state()

	// 操作指针对应的值. state是指针指向的新值()
	state := atomic.AddUint64(statep, uint64(delta)<<32)
	v := int32(state >> 32) // 高32位对应的值(计数值, 指用来统计goroutine当前数量的值)
	w := uint32(state)      // 低32位对应的值(等待计数)

	if v < 0 {
		panic("sync: negative WaitGroup counter")
	}

	// w != 0, 当前的等待计数大于0
	// delta > 0, 添加新的goroutine
	// v == int32(delta), 计数器
	if w != 0 && delta > 0 && v == int32(delta) {
		panic("sync: WaitGroup misuse: Add called concurrently with Wait")
	}
	if v > 0 || w == 0 {
		return
	}

	// 当等待计数器 > 0 时,而goroutine设置为0
	// 此时不可能有同时发生的状态突变:
	// - 增加不能与等待同时发生
	// - 如果计数器counter == 0, 不再增加等待计数器
	if *statep != state {
		panic("sync: WaitGroup misuse: Add called concurrently with Wait")
	}
	// Reset waiters count to 0.
	*statep = 0
	for ; w != 0; w-- {
		// 目的是一个简单的wakeup原语, 以供同步使用. true为唤醒排在等待队列的第一个goroutine
		//runtime_Semrelease(&wg.sema, false)
	}
}

func (wg *WaitGroup) Wait() {
	statep := wg.state()
	// csa算法
	for {
		state := atomic.LoadUint64(statep) // 加载statep指针指向的值
		v := int32(state >> 32) // 计数器的值
		//w := uint32(state) // 等待计数的值
		if v == 0 {
			return
		}
		// 增加等待的goroutine数量, 对低32为数加1
		if atomic.CompareAndSwapUint64(statep, state, state+1) {
			// 目的是一个简单的sleep原语, 以供同步使用
			//runtime_Semacquire(&wg.sema)
			if *statep != 0 {
				panic("sync: WaitGroup is reused before previous Wait has returned")
			}
			return
		}
	}
}
