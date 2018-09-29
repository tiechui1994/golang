package goroutines

import (
	"testing"
	"time"
	"strings"
	"fmt"
)

func TestWorker(t *testing.T) {
	Run()
}

func TestPublisher(t *testing.T) {
	p := NewPublisher(100*time.Millisecond, 10)
	defer p.Close()

	all := p.Subscribe()
	golang := p.SubscribeTopic(func(v interface{}) bool {
		if s, ok := v.(string); ok {
			return strings.Contains(s, "golang")
		}
		return false
	})

	p.Publish("Hello, world")
	p.Publish("hello, golang")

	go func() {
		for msg := range all {
			fmt.Println("all:", msg)
		}
	}()

	go func() {
		for msg := range golang {
			fmt.Println("golang:", msg)
		}
	}()

	time.Sleep(3 * time.Second)
}

// 素数筛, 每个并发体处理的任务粒度太细, 程序的整体性能并不理想
func TestPrime(t *testing.T) {
	var (
		generateNatural = func() chan int {
			ch := make(chan int)
			go func() {
				for i := 2; ; i++ {
					ch <- i
				}
			}()
			return ch
		}

		primeFilter = func(in <-chan int, prime int) chan int {
			out := make(chan int)
			go func() {
				for {
					if i := <-in; i%prime != 0 {
						out <- i
					}
				}
			}()
			return out
		}
	)

	ch := generateNatural()
	for i := 0; i < 1000; i++ {
		prime := <-ch
		fmt.Printf("%v: %v\n", i+1, prime)
		ch = primeFilter(ch, prime)
	}
}
