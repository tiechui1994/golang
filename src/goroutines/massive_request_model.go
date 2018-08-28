package goroutines

import (
	"sync"
	"fmt"
)

/*
消费海量的请求:
	WorkerPool
*/

type Worker struct {
	job  chan interface{}
	quit chan bool
	wg   sync.WaitGroup
}

func NewWorker(maxJobs int) *Worker {
	return &Worker{
		job:  make(chan interface{}, maxJobs),
		quit: make(chan bool),
	}
}

func (w *Worker) Start() {
	w.wg.Add(1)

	go func() {
		defer w.wg.Done()

		for {
			// 接收任务
			// 此时Worker已经从WorkerPool当中取出
			select {
			case job := <-w.job:
				// 处理任务
				fmt.Println(job)
			case <-w.quit:
				return
			}
		}
	}()
}

func (w *Worker) Stop() {
	w.quit <- true
	w.wg.Wait()
}

func (w *Worker) AddJob(job interface{}) {
	w.job <- job
}

type WorkerPool struct {
	workers []*Worker
	pool    chan *Worker
}

func NewWorkerPool(maxWorkers int) *WorkerPool {
	pool := &WorkerPool{
		workers: make([]*Worker, maxWorkers),
		pool:    make(chan *Worker, maxWorkers),
	}

	for i := range pool.workers {
		worker := NewWorker(0)
		pool.workers[i] = worker
		pool.pool <- worker
	}

	return pool
}

func (wp *WorkerPool) Start() {
	for _, worker := range wp.workers {
		worker.Start()
	}
}

func (wp *WorkerPool) Stop() {
	for _, worker := range wp.workers {
		worker.Stop()
	}
}

func (wp *WorkerPool) Get() *Worker {
	return <-wp.pool
}

func (wp *WorkerPool) Put(w *Worker) {
	wp.pool <- w
}

type Service struct {
	workers *WorkerPool
	jobs    chan interface{}
	maxJobs int
	wg      sync.WaitGroup
}

func NewService(maxWorkers, maxJobs int) *Service {
	return &Service{
		workers: NewWorkerPool(maxWorkers),
		jobs:    make(chan interface{}, maxJobs),
		maxJobs: maxJobs,
	}
}

func (s *Service) Start() {
	s.jobs = make(chan interface{}, s.maxJobs)
	s.wg.Add(1)
	s.workers.Start()

	go func() {
		defer s.wg.Done()

		for job := range s.jobs {
			go func(job interface{}) {
				// 从工作池取出一个Worker
				worker := s.workers.Get()
				// 完成任务后返回给工作池
				defer s.workers.Put(worker)
				// 提交任务处理(异步)
				worker.AddJob(job)
			}(job)
		}
	}()
}

func (s *Service) Stop()  {
	s.workers.Stop()
	close(s.jobs)
	s.wg.Wait()
}

func (s *Service) AddJob(job interface{})  {
	s.jobs <- job
}
