package details

import "fmt"

// slice, array, pointer 传值的对比
// array: 值复制, 底层的指针发生改变
// slice: 值复制, 但是底层的指针没有改变
// pointer: 无拷贝
func TransferValue() {
	a := [3]int{1, 2, 3}
	b := &[3]int{1, 2, 3}
	c := []int{1, 2, 3}

	farray := func(x [3]int) {
		x[0] = 100
		fmt.Printf("array value: %v \n", &x == &a)
	}

	fpointer := func(x *[3]int) {
		x[0] = 100
		fmt.Printf("pointer value: %v \n", x == b)
	}

	fslice := func(x []int) {
		x[0] = 100
		fmt.Printf("slice value: %v \n", &x == &c)
	}

	farray(a)
	fmt.Printf("a: %+v \n", a)

	fpointer(b)
	fmt.Printf("b: %+v \n", b)

	fslice(c)
	fmt.Printf("c: %+v \n", c)
}
