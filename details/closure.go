package details

import (
	"fmt"
	"reflect"
	"time"
)

/**

闭包: 内层函数引用了外层函数中的变量, 其返回值也是一个函数.

条件:
	1. 内层函数(匿名函数), 外层函数
	2. 自由变量
	3. 返回值是函数

func outer() func() {
	var x int

	return func() {
		x+=1
	}
}

自由变量的生命周期被延长和外层函数的生命周期一致.


常见的坑:(后面有案例)
1. for range中使用闭包
2. 函数列表使用不当
3. 延迟调用

**/
func makeEvenGenerator() func() uint {
	i := uint(0)

	return func() (ret uint) {
		ret = i
		i += 2
		return
	}
}

func ClosureVar() {
	nextEven := makeEvenGenerator()
	fmt.Println(reflect.TypeOf(nextEven))
	fmt.Println(nextEven())
	fmt.Println(nextEven())
}

// 没有将变量token拷贝值传进匿名函数之前, 只能获取最后一次循环的值.
func ClosureForRange() {
	tokens := []string{"a", "b", "c"}

	for _, token := range tokens {
		go func() {
			fmt.Println(token) // 结果: ???
		}()
	}

	/*
	// 改进方案
	for _, token := range tokens {
		go func(token string) {
			fmt.Println(token) // 结果: ???
		}(token)
	}
	*/

	// 阻塞模式
	select {
	case <-time.After(500 * time.Millisecond):
	}
}

// 每次append操作仅将匿名 函数放入到列表当中, 单并未执行, 并且引用的变量都是i,
// 随着i的改变匿名函数中的i也在改变, 所以当执行这些函数时, 读取的都是i最后一次值
func FuncList() []func() {
	var list []func() // 匿名函数列表

	for i := 0; i < 3; i++ {
		list = append(list, func() {
			fmt.Println(&i, i)
		})
	}

	/*
	// 改进方案
	for i := 0; i < 3; i++ {
		x := i
		list = append(list, func() {
			fmt.Println(&x, x)
		})
	}
	*/
	return list
}
func ClosureList() {
	for _, f := range FuncList() {
		f()
	}
}

// defer 调用会在当前函数执行结束前才被执行, 这些调用称为延迟调用, defer中使用匿名函数依然是
// 一个闭包
// 原因参考func.go当中defer参数解析
func ClosureDefer() {
	x, y := 1, 2

	defer func(a int) {
		fmt.Printf("x:%d, y:%d \n", a, y)
	}(x)

	x += 100
	y += 100
	fmt.Println(x, y)
}
