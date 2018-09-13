package goroutines

import (
	"testing"
	"fmt"
	"sync"
)

func TestChannelGroup(t *testing.T) {
	cg := NewChannelGroup()
	for i := 0; i < 10; i++ {
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
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(i int) {
			fmt.Println("i: ", i)
			wg.Done()
		}(i)
	}
	wg.Wait()
}
