package random

import (
	mathRand "math/rand"
	"time"
	"fmt"
)

/**
math/rand 包实现了伪随机数生成器. 也就是生成整形和浮点型.
　　该包中根据生成伪随机数是是否有种子(可以理解为初始化伪随机数),可以分为两类:
　　1、有种子. 通常以时钟,输入输出等特殊节点作为参数进行初始化. 该类型生成的随机数相比无种子时重复概率较低.
　　2、无种子. 可以理解为此时种子为1, 即Seek(1).
*/
func MathRandom() {
	seedRand := mathRand.New(mathRand.NewSource(time.Now().UnixNano()))
	for i := 0; i < 100; i++ {
		fmt.Println(seedRand.Int())
	}

	for i := 0; i < 100; i++ {
		fmt.Println(mathRand.Int())
	}
}

/**
crypto/rand包实现了用于加解密的更安全的随机数生成器.
　　该包中常用的是 Read(b []byte) (n int, err error) 这个方法,将随机的byte值填充到b数组中.
*/