package pool_test

import (
	"sync"
	"testing"
	"time"

	"golang/pool"
)

const (
	RunTimes      = 10000000
	benchParam    = 10
	benchPoolSize = 100000
)

func demoFunc() error {
	n := 10
	time.Sleep(time.Duration(n) * time.Millisecond)
	return nil
}

func demoPoolFunc(args interface{}) error {
	//m := args.(int)
	//var n int
	//for i := 0; i < m; i++ {
	//	n += i
	//}
	//return nil
	n := args.(int)
	time.Sleep(time.Duration(n) * time.Millisecond)
	return nil
}

func BenchmarkGoroutineWithFunc(b *testing.B) {
	var wg sync.WaitGroup
	for i := 0; i < b.N; i++ {
		wg.Add(RunTimes)
		for j := 0; j < RunTimes; j++ {
			go func() {
				demoPoolFunc(benchParam)
				wg.Done()
			}()
		}
		wg.Wait()
	}
}

func BenchmarkSemaphoreWithFunc(b *testing.B) {
	var wg sync.WaitGroup
	sema := make(chan struct{}, benchPoolSize)

	for i := 0; i < b.N; i++ {
		wg.Add(RunTimes)
		for j := 0; j < RunTimes; j++ {
			sema <- struct{}{}
			go func() {
				demoPoolFunc(benchParam)
				<-sema
				wg.Done()
			}()
		}
		wg.Wait()
	}
}

func BenchmarkPoolWithFunc(b *testing.B) {
	var wg sync.WaitGroup
	p, _ := pool.NewPool(benchPoolSize)
	defer p.Close()

	b.StartTimer()
	job, _ := pool.NewJob(demoPoolFunc, benchParam)
	for i := 0; i < b.N; i++ {
		wg.Add(RunTimes)
		for j := 0; j < RunTimes; j++ {
			p.Submit(job)
		}
		wg.Wait()
		//b.Logf("running concurrent: %d", p.Running())
	}
	b.StopTimer()
}

func BenchmarkGoroutine(b *testing.B) {
	for i := 0; i < b.N; i++ {
		for j := 0; j < RunTimes; j++ {
			go demoPoolFunc(benchParam)
		}
	}
}

func BenchmarkSemaphore(b *testing.B) {
	sema := make(chan struct{}, benchPoolSize)
	for i := 0; i < b.N; i++ {
		for j := 0; j < RunTimes; j++ {
			sema <- struct{}{}
			go func() {
				demoPoolFunc(benchParam)
				<-sema
			}()
		}
	}
}

func BenchmarkPool(b *testing.B) {
	p, _ := pool.NewPool(benchPoolSize)
	defer p.Close()
	b.StartTimer()
	job, _ := pool.NewJob(demoPoolFunc, benchParam)
	for i := 0; i < b.N; i++ {
		for j := 0; j < RunTimes; j++ {
			p.Submit(job)
		}
	}
	b.StopTimer()
}
