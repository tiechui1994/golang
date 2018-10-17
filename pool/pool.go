package pool

import (
	"sync"
	"sync/atomic"
	"time"
)

type sig struct{}

type Pool struct {
	capacity int32 // Pool的容量, 即开启worker数量的上限, 每一个worker绑定一个goroutine.
	running  int32 // 当前正在执行任务的worker数量

	expiryDuration time.Duration // 每个协程的运行时长(second)

	idleWorkers []*Worker // 存放空闲worker

	signal chan sig // 关闭协程池的信号

	lock sync.Mutex // 锁,用以支持Pool的同步操作
	cond *sync.Cond // 唤醒操作

	once sync.Once
}

// time.NewTicker() 定时器, 每间隔 d 向Ticker当中的管道C发送当时的时间
// time.NewTimer()  定时器, 经过时间 d 之后, 向Timer当中的管道C发送当前时间
func (p *Pool) periodicallyPurge() {
	// 心跳检测空闲worker
	heartbeat := time.NewTicker(p.expiryDuration)
	for range heartbeat.C { // 周期性阻塞,
		currentTime := time.Now()
		p.lock.Lock()
		idleWorkers := p.idleWorkers
		// 空闲的Worker为0, 运行的Worker为0, 且发送关闭信号
		if len(idleWorkers) == 0 && p.Running() == 0 && len(p.signal) > 0 {
			p.lock.Unlock()
			return
		}
		n := -1
		for i, w := range idleWorkers {
			if currentTime.Sub(w.recycleTime) <= p.expiryDuration {
				break
			}
			n = i
			w.job <- nil
			idleWorkers[i] = nil
		}
		if n > -1 {
			if n >= len(idleWorkers)-1 {
				p.idleWorkers = idleWorkers[:0]
			} else {
				p.idleWorkers = idleWorkers[n+1:]
			}
		}
		p.lock.Unlock()
	}
}

func NewPool(capacity int) (*Pool, error) {
	return NewTimingPool(capacity, DefaultCleanInterval)
}

// 自定义协程池
func NewTimingPool(capacity, expiry int) (*Pool, error) {
	if capacity <= 0 {
		return nil, ErrInvalidPoolSize
	}
	if expiry <= 0 {
		return nil, ErrInvalidPoolExpiry
	}
	p := &Pool{
		capacity:       int32(capacity),
		signal:         make(chan sig, 1),
		expiryDuration: time.Duration(expiry) * time.Second,
	}
	p.cond = sync.NewCond(&p.lock)
	go p.periodicallyPurge()
	return p, nil
}

//-------------------------------------------------------------------------

// 提交任务
func (p *Pool) Submit(job *job) error {
	if len(p.signal) > 0 {
		return ErrPoolClosed
	}

	p.getWorker().job <- job

	return nil
}

func (p *Pool) Running() int {
	return int(atomic.LoadInt32(&p.running))
}

func (p *Pool) Idle() int {
	return int(atomic.LoadInt32(&p.capacity) - atomic.LoadInt32(&p.running))
}

func (p *Pool) Cap() int {
	return int(atomic.LoadInt32(&p.capacity))
}

// 重置Pool的容量
func (p *Pool) ResetCap(capacity int) {
	if capacity == p.Cap() {
		return
	}
	atomic.StoreInt32(&p.capacity, int32(capacity))
	diff := p.Running() - capacity
	for i := 0; i < diff; i++ {
		p.getWorker().job <- nil
	}
}

// 关闭
func (p *Pool) Close() error {
	p.once.Do(func() {
		p.signal <- sig{}
		p.lock.Lock()
		defer p.lock.Unlock()

		idleWorkers := p.idleWorkers
		for i, w := range idleWorkers {
			w.job <- nil
			idleWorkers[i] = nil
		}
		p.idleWorkers = nil
	})
	return nil
}

//-------------------------------------------------------------------------

// 增加运行的Worker数量
func (p *Pool) incRunning() {
	atomic.AddInt32(&p.running, 1)
}

// 减少运行的Worker数量
func (p *Pool) decRunning() {
	atomic.AddInt32(&p.running, -1)
}

// 获取一个Worker, 调度算法的核心
func (p *Pool) getWorker() *Worker {
	var w *Worker
	waiting := false

	p.lock.Lock()
	defer p.lock.Unlock()

	idleWorkers := p.idleWorkers
	n := len(idleWorkers) - 1
	if n < 0 {
		// 没有空闲的Wotker, 检查是否需要等待
		waiting = p.Running() >= p.Cap()
	} else {
		// 有空闲的Worker, 取出空闲的Worker
		w = idleWorkers[n]
		idleWorkers[n] = nil
		p.idleWorkers = idleWorkers[:n]
	}

	if waiting {
		for {
			p.cond.Wait() // 等待
			l := len(p.idleWorkers) - 1
			if l < 0 {
				continue
			}
			w = p.idleWorkers[l]
			p.idleWorkers[l] = nil
			p.idleWorkers = p.idleWorkers[:l]
			break
		}
	} else if w == nil { // 创建一个新的Worker
		w = &Worker{
			pool: p,
			job:  make(chan *job, 1),
		}
		w.run()
		p.incRunning()
	}

	return w
}

// 回收Worker
func (p *Pool) putWorker(worker *Worker) {
	worker.recycleTime = time.Now() // 设置Worker的回收时间
	p.lock.Lock()
	p.idleWorkers = append(p.idleWorkers, worker)
	p.cond.Signal() // 通知有一个空闲的worker
	p.lock.Unlock()
}
