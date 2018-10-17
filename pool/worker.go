package pool

import (
	"time"
	"reflect"
)

type Job struct {
	function interface{}
	args     []interface{}
}

func NewJob(function interface{}, args ...interface{}) *Job {
	var (
		val = reflect.ValueOf(function)
		typ = reflect.TypeOf(function)
	)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
		typ = typ.Elem()
	}

	if val.Kind() != reflect.Func {
		panic("function type is invalid")
	}

	if typ.NumIn() != len(args) {
		panic("function params is invalid")
	}

	return &Job{
		function: val.Interface(),
		args:     args,
	}
}

func (f *Job) Execute() {
	fun := reflect.ValueOf(f.function)
	args := make([]reflect.Value, len(f.args))

	for k, v := range f.args {
		args[k] = reflect.ValueOf(v)
	}

	fun.Call(args)
}

type Worker struct {
	pool *Pool

	job chan *Job

	recycleTime time.Time // 在将空闲的worker放回到空闲列表当中, recycleTime更新为当前的时间
}

func (w *Worker) run() {
	go func() {
		for f := range w.job {
			if f == nil {
				w.pool.decRunning()
				return
			}

			f.Execute()
			w.pool.putWorker(w)
		}
	}()
}
