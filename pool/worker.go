package pool

import (
	"time"
	"reflect"
)

type job struct {
	function interface{}
	args     []interface{}
}

func NewJob(function interface{}, args ...interface{}) (*job, error) {
	var (
		val = reflect.ValueOf(function)
		typ = reflect.TypeOf(function)
	)
	if val.Kind() == reflect.Ptr {
		val = val.Elem()
		typ = typ.Elem()
	}

	if val.Kind() != reflect.Func {
		return nil, ErrFunction
	}

	if typ.NumIn() != len(args) {
		return nil, ErrFunctionArgs
	}

	return &job{
		function: val.Interface(),
		args:     args,
	}, nil
}

func (f *job) Execute() {
	fun := reflect.ValueOf(f.function)
	args := make([]reflect.Value, len(f.args))

	for k, v := range f.args {
		args[k] = reflect.ValueOf(v)
	}

	fun.Call(args)
}

type Worker struct {
	pool *Pool

	job chan *job

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
