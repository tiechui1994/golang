package lib

func init() {
	println("sum.init")
}

func Sum(args ...int) int {
	sum := 0
	for _, v := range args {
		sum += v
	}

	return sum
}
