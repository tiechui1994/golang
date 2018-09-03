package goroutines

import (
	"sync"
	"sync/atomic"
)

type singleton struct {
}

var (
	instance    *singleton
	initialized uint32
	mutex       sync.Mutex
)

func Instance() *singleton {
	if atomic.LoadUint32(&initialized) == 1 { // 读取initialized的值, 存在则已经初始化
		return instance
	}

	mutex.Lock()
	defer mutex.Unlock()

	if instance == nil {
		defer atomic.StoreUint32(&initialized, 1) // 设置初始化标记
		instance = &singleton{}
	}
	return instance
}

/*
	sync.Once实现. Do方法保证当前的函数加载的时候只执行一次
*/
type Once struct {
	mutex sync.Mutex
	done  uint32
}

func (o *Once) Do(f func()) {
	if atomic.LoadUint32(&o.done) == 1 {
		return
	}

	o.mutex.Lock()
	defer o.mutex.Unlock()

	if o.done == 0 {
		defer atomic.StoreUint32(&o.done, 1)
		f()
	}
}

/*
	基于sync.Once的Singleton
*/
var (
	inst *singleton
	once Once
)

func OnceInstance() *singleton {
	once.Do(func() {
		inst = &singleton{}
	})
	return inst
}
