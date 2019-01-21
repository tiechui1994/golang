package pool_test

import (
	"fmt"
	"golang/pool"
	"runtime"
	"sync"
	"testing"
	"time"
)

const (
	_   = 1 << (10 * iota)
	KiB // 1024
	MiB // 1048576
	GiB // 1073741824
	TiB // 1099511627776             (超过了int32的范围)
	PiB // 1125899906842624
	EiB // 1152921504606846976
	ZiB // 1180591620717411303424    (超过了int64的范围)
	YiB // 1208925819614629174706176
)

const (
	Param    = 100
	PoolSize = 1000
	TestSize = 10000
	n        = 100
)

var curMem uint64

func TestNoPool(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			demoFunc()
			wg.Done()
		}()
	}

	wg.Wait()
	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

func TestPool(t *testing.T) {
	defer pool.Close()
	var wg sync.WaitGroup
	for i := 0; i < n; i++ {
		wg.Add(1)
		job, _ := pool.NewJob(
			func() error {
				demoFunc()
				wg.Done()
				return nil
			},
		)
		pool.Submit(job)
	}
	wg.Wait()

	t.Logf("pool, capacity:%d", pool.Cap())
	t.Logf("pool, running workers number:%d", pool.Running())
	t.Logf("pool, idel workers number:%d", pool.Idle())

	mem := runtime.MemStats{}
	runtime.ReadMemStats(&mem)
	curMem = mem.TotalAlloc/MiB - curMem
	t.Logf("memory usage:%d MB", curMem)
}

func TestCodeCov(t *testing.T) {
	_, err := pool.NewTimingPool(-1, -1)
	t.Log(err)
	_, err = pool.NewTimingPool(1, -1)
	t.Log(err)

	job, _ := pool.NewJob(demoFunc)
	p0, _ := pool.NewPool(PoolSize)
	defer p0.Submit(job)
	defer p0.Close()
	for i := 0; i < n; i++ {
		p0.Submit(job)
	}
	t.Logf("pool, capacity:%d", p0.Cap())
	t.Logf("pool, running workers number:%d", p0.Running())
	t.Logf("pool, free workers number:%d", p0.Idle())
	p0.ResetCap(PoolSize)
	p0.ResetCap(PoolSize / 2)
	t.Logf("pool, after resize, capacity:%d, running:%d", p0.Cap(), p0.Running())

	p, _ := pool.NewPool(TestSize)
	defer p.Submit(job)
	defer p.Close()
	for i := 0; i < n; i++ {
		p.Submit(job)
	}
	time.Sleep(pool.DefaultCleanInterval * time.Second)
	t.Logf("pool with func, capacity:%d", p.Cap())
	t.Logf("pool with func, running workers number:%d", p.Running())
	t.Logf("pool with func, free workers number:%d", p.Idle())
	p.ResetCap(TestSize)
	p.ResetCap(PoolSize)
	t.Logf("pool with func, after resize, capacity:%d, running:%d", p.Cap(), p.Running())
}
