package concurrent

import (
	"fmt"
	"sync"
	"testing"
)

/*
并发控制的模式:
	方式一: 使用channel
	方式二: 使用WaitGroup
*/

func TestChannelGroup(t *testing.T) {
	cg := NewChannelGroup()
	for i := 0; i < 1800000; i++ {
		cg.Add(1)
		go func(i int) {
			fmt.Println("ci: ", i)
			cg.Done()
		}(i)
	}
	cg.Wait()
}

func TestWaitGroup(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 1559999; i++ {
		wg.Add(1)
		go func(i int) {
			fmt.Println("i: ", i)
			wg.Done()
		}(i)
	}
	wg.Wait()
}
