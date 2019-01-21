package synch

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
)

/*
典型的死锁检测:
	exited: 退出的信号
	wg1: 每个协程等待 wg2 的完成, 一旦完成,则发送一个退出信号
	wg2: 测试 wg1 当中发出的退出信号
*/
func testWaitGroup(t *testing.T, wg1 *sync.WaitGroup, wg2 *sync.WaitGroup) {
	n := 16
	exited := make(chan bool, n)
	wg1.Add(n)
	wg2.Add(n)
	for i := 0; i != n; i++ {
		go func() {
			wg1.Done()
			wg2.Wait() // 阻塞,等待所有wg2的完成
			exited <- true
		}()
	}
	wg1.Wait() // 阻塞,保证 wg1的所有协程已经开启
	for i := 0; i != n; i++ {
		select {
		case <-exited:
			t.Fatal("WaitGroup released group too soon")
		default:
		}
		wg2.Done()
	}
	for i := 0; i != n; i++ {
		<-exited // Will block if barrier fails to unlock someone.
	}
}

// 死锁检查
func TestWaitGroup(t *testing.T) {
	wg1 := &sync.WaitGroup{}
	wg2 := &sync.WaitGroup{}

	// Run the same test a few times to ensure barrier is in a proper state.
	for i := 0; i != 8; i++ {
		testWaitGroup(t, wg1, wg2)
	}
}

func knownRacy(t *testing.T) {
	var enabled bool
	if enabled {
		t.Skip("skipping known-racy test under the race detector")
	}
}

/*
wg.Add() 向wg当中添加协程, 纳入wg的管理
wg.Done() 协程工作完成,从wg当中移除. 当wg当中没有协程时候, 再调用此方法会抛出异常
wg.Wait() 等待wg所有协程的完成. 阻塞调用

wg类似一个队列, Add() 向队列当中添加元素, Done()从队列当中移除元素, Wait()等待队列为空

wg使用不当:
	wg.Add()添加的总数 < wg.Done()调用的次数 => 抛出异常("sync: negative WaitGroup counter")
*/
func TestWaitGroupMisuse(t *testing.T) {
	defer func() {
		err := recover()
		if err != "sync: negative WaitGroup counter" {
			t.Fatalf("Unexpected panic: %#v", err)
		}
	}()
	wg := &sync.WaitGroup{}
	wg.Add(1)
	wg.Done()
	wg.Done() // 抛出异常
	t.Fatal("Should panic")
}

/*
wg使用不当:
	使用Wait()和Add()并发调用. 案例当中(1),(2),(3) 这三者是并发调用的
*/
func TestWaitGroupMisuse2(t *testing.T) {
	knownRacy(t)
	//if runtime.NumCPU() <= 4 {
	//	t.Skip("NumCPU<=4, skipping: this test requires parallelism")
	//}
	defer func() {
		err := recover()
		_, file, line, _ := runtime.Caller(3)
		fmt.Println(file, line)
		fmt.Println(err)
		if err != "sync: negative WaitGroup counter" &&
			err != "sync: WaitGroup misuse: Add called concurrently with Wait" &&
			err != "sync: WaitGroup is reused before previous Wait has returned" {
			t.Fatalf("Unexpected panic: %#v", err)
		}
	}()
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))
	done := make(chan interface{}, 2) // 信号

	// 这种检测是带有机会性的, 期待在一次运行l00万协程的状况下发生异常
	for i := 0; i < 1e6; i++ {
		var wg sync.WaitGroup
		var here uint32
		wg.Add(1)
		go func() {
			defer func() {
				done <- recover() // 期待异常
			}()
			atomic.AddUint32(&here, 1)
			for atomic.LoadUint32(&here) != 3 {
				// spin
			}
			wg.Wait() // (1)
		}()
		go func() {
			defer func() {
				done <- recover() // 期待异常(异常发生的地方)
			}()
			atomic.AddUint32(&here, 1)
			for atomic.LoadUint32(&here) != 3 {
				// spin
			}
			wg.Add(1) // bad操作 (2)
			wg.Done()
		}()
		atomic.AddUint32(&here, 1)
		for atomic.LoadUint32(&here) != 3 {
			// spin
		}
		wg.Done() // (3)

		for j := 0; j < 2; j++ {
			if err := <-done; err != nil {
				panic(err)
			}
		}
	}
	t.Fatal("Should panic")
}

/*
wg使用不当:
   	WaitGroup在 "previous Wait()返回之前" 被重用, (1), (2), (3)之间Wait()关系
*/
func TestWaitGroupMisuse3(t *testing.T) {
	knownRacy(t)
	if runtime.NumCPU() <= 1 {
		t.Skip("NumCPU==1, skipping: this test requires parallelism")
	}
	defer func() {
		err := recover()
		fmt.Println(err)
		if err != "sync: negative WaitGroup counter" &&
			err != "sync: WaitGroup misuse: Add called concurrently with Wait" &&
			err != "sync: WaitGroup is reused before previous Wait has returned" {
			t.Fatalf("Unexpected panic: %#v", err)
		}
	}()
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(4))
	done := make(chan interface{}, 2)

	// 这种检测是带有机会性的, 期待在一次运行l00万协程的状况下发生异常
	for i := 0; i < 1; i++ {
		var wg sync.WaitGroup
		wg.Add(1)

		go func() {
			defer func() {
				done <- recover()
			}()
			wg.Done()
		}()

		go func() {
			defer func() {
				done <- recover()
			}()
			wg.Wait() // (1) previous Wait() 是 (3)

			// 重用wg
			wg.Add(1)
			go func() {
				wg.Done()
			}()
			wg.Wait() // (2) previous Wait() 是 (1)
		}()

		wg.Wait() // (3)

		for j := 0; j < 2; j++ {
			if err := <-done; err != nil {
				panic(err)
			}
		}
	}
	t.Fatal("Should panic")
}

func BenchmarkWaitGroupUncontended(b *testing.B) {
	type PaddedWaitGroup struct {
		sync.WaitGroup
		pad [128]uint8
	}
	b.RunParallel(func(pb *testing.PB) {
		var wg PaddedWaitGroup
		for pb.Next() {
			wg.Add(1)
			wg.Done()
			wg.Wait()
		}
	})
}

// 压测: Add() 与 Done(), 使用一个wg
func benchmarkWaitGroupAddDone(b *testing.B, localWork int) {
	var wg sync.WaitGroup
	b.RunParallel(func(pb *testing.PB) {
		foo := 0
		for pb.Next() {
			wg.Add(1)
			for i := 0; i < localWork; i++ {
				foo *= 2
				foo /= 2
			}
			wg.Done()
		}
		_ = foo
	})
}

func BenchmarkWaitGroupAddDone(b *testing.B) {
	benchmarkWaitGroupAddDone(b, 0)
}

func BenchmarkWaitGroupAddDoneWork(b *testing.B) {
	benchmarkWaitGroupAddDone(b, 100)
}

// 压测: Wait(), 使用一个wg
func benchmarkWaitGroupWait(b *testing.B, localWork int) {
	var wg sync.WaitGroup
	b.RunParallel(func(pb *testing.PB) {
		foo := 0
		for pb.Next() {
			wg.Wait()
			for i := 0; i < localWork; i++ {
				foo *= 2
				foo /= 2
			}
		}
		_ = foo
	})
}

func BenchmarkWaitGroupWait(b *testing.B) {
	benchmarkWaitGroupWait(b, 0)
}

func BenchmarkWaitGroupWaitWork(b *testing.B) {
	benchmarkWaitGroupWait(b, 100)
}

// 压测: 真实状况下的 Add() -> Done() -> Wait()
func BenchmarkWaitGroupActuallyWait(b *testing.B) {
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			var wg sync.WaitGroup
			wg.Add(1)
			go func() {
				wg.Done()
			}()
			wg.Wait()
		}
	})
}
