package details

import (
	"fmt"
	"reflect"
)

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
	fmt.Println(nextEven())
}
