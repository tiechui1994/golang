package future

import (
	"sync/atomic"
	"unsafe"
	"time"
	"fmt"
	"errors"
)

type callbackType int

const (
	CALLBACK_DONE   callbackType = iota
	CALLBACK_FAIL
	CALLBACK_ALWAYS
	CALLBACK_CANCEL
)

// pip 是 Promise的链式执行
type pipe struct {
	pipeDoneTask, pipeFailTask func(v interface{}) *Future
	pipePromise                *Promise
}

// getPipe returns piped Future task function and pipe Promise by the status of current Promise.
func (future *pipe) getPipe(isResolved bool) (func(v interface{}) *Future, *Promise) {
	if isResolved {
		return future.pipeDoneTask, future.pipePromise
	} else {
		return future.pipeFailTask, future.pipePromise
	}
}

// 检查Future是否被取消
// 它通常被传递给Future任务函数, Future任务函数可以检查Future是否被取消
type Canceller interface {
	IsCancelled() bool
	Cancel()
}

type canceller struct {
	f *Future
}

// 将 Future 的状态设置为 CANCELLED
func (future *canceller) Cancel() {
	future.f.Cancel()
}

// 确定Future的状态是否被取消
func (future *canceller) IsCancelled() (r bool) {
	return future.f.IsCancelled()
}

// 存储最终Future的状态
type futureVal struct {
	dones, fails, always []func(v interface{})
	cancels              []func()
	pipes                []*pipe
	result               *PromiseResult
}

// Future 提供的是一个只读的Promise的视图. 它的值在调用Promise的 Resolve | Reject | Cancel 方法之后被确定
type Future struct {
	Id    int // Future的唯一标识
	final chan struct{}

	// val是 futureVal的一个指针.
	// 如果需要修改Future的状态, 必须先copy一个新的futureVal, 并修改它的值, 然后使用CAS将这新的futureVal设置给val
	val unsafe.Pointer
}

// Canceller returns a canceller object related to future.
func (future *Future) Canceller() Canceller {
	return &canceller{future}
}

//  Point -> Object -> Field
func (future *Future) loadResult() *PromiseResult {
	val := future.loadVal()
	return val.result
}

// Point -> Object的转换
func (future *Future) loadVal() *futureVal {
	r := atomic.LoadPointer(&future.val)
	return (*futureVal)(r)
}

func (future *Future) IsCancelled() bool {
	val := future.loadVal()

	if val != nil && val.result != nil && val.result.Type == RESULT_CANCELLED {
		return true
	} else {
		return false
	}
}

// 设置Future的超时时间, 单位ms
func (future *Future) SetTimeout(mm int) *Future {
	if mm == 0 {
		mm = 10
	} else {
		mm = mm * 1000 * 1000
	}

	go func() {
		<-time.After((time.Duration)(mm) * time.Nanosecond)
		future.Cancel()
	}()
	return future
}

// 返回一个 chan PromiseResult, 是Promise的结果
func (future *Future) GetChan() <-chan *PromiseResult {
	c := make(chan *PromiseResult, 1)
	future.OnComplete(func(v interface{}) {
		c <- future.loadResult()
	}).OnCancel(func() {
		c <- future.loadResult()
	})
	return c
}

// todo: 获取 Future 的结果, 一直阻塞调用, 直到有结果返回, 需要重点解析 getFutureReturnVal() 方法
func (future *Future) Get() (val interface{}, err error) {
	<-future.final
	return getFutureReturnVal(future.loadResult())
}

// todo: 类似Get()方法, 阻塞的时间最多是 mm 毫秒, 就会有返回结果
func (future *Future) GetOrTimeout(mm uint) (val interface{}, err error, timout bool) {
	if mm == 0 {
		mm = 10
	} else {
		mm = mm * 1000 * 1000
	}

	select {
	case <-time.After((time.Duration)(mm) * time.Nanosecond):
		return nil, nil, true
	case <-future.final:
		r, err := getFutureReturnVal(future.loadResult())
		return r, err, false
	}
}

// 设置 Promise 的状态为 RESULT_CANCELLED
func (future *Future) Cancel() (e error) {
	return future.setResult(&PromiseResult{CANCELLED, RESULT_CANCELLED})
}

// 注册成功返回的回调函数
func (future *Future) OnSuccess(callback func(v interface{})) *Future {
	future.addCallback(callback, CALLBACK_DONE)
	return future
}

// 注册失败返回的回调函数
func (future *Future) OnFailure(callback func(v interface{})) *Future {
	future.addCallback(callback, CALLBACK_FAIL)
	return future
}

// 注册 Promise 有返回(不论成功或者失败)结果的回调函数
func (future *Future) OnComplete(callback func(v interface{})) *Future {
	future.addCallback(callback, CALLBACK_ALWAYS)
	return future
}

// 注册Promise取消的回调函数
func (future *Future) OnCancel(callback func()) *Future {
	future.addCallback(callback, CALLBACK_CANCEL)
	return future
}

// 注册 一个或者两个回调函数. 并且返回 代理的 Futura
// 当Future成功返回, 第一个回调函数被调用
// 当Future失败返回, 第二个回调函数被调用
func (future *Future) Pipe(callbacks ...interface{}) (result *Future, ok bool) {
	if len(callbacks) == 0 ||
		(len(callbacks) == 1 && callbacks[0] == nil) ||
		(len(callbacks) > 1 && callbacks[0] == nil && callbacks[1] == nil) {
		result = future
		return
	}

	//todo: 验证回调函数的格式 "func(v interface{}) *Future"
	cs := make([]func(v interface{}) *Future, len(callbacks), len(callbacks))
	for i, callback := range callbacks {
		if function, status := callback.(func(v interface{}) *Future); status {
			cs[i] = function

		} else if function, status := callback.(func() *Future); status {
			cs[i] = func(v interface{}) *Future {
				return function()
			}

		} else if function, status := callback.(func(v interface{})); status {
			cs[i] = func(v interface{}) *Future {
				return Start(func() {
					function(v)
				})
			}

		} else if function, status := callback.(func(v interface{}) (r interface{}, err error)); status {
			cs[i] = func(v interface{}) *Future {
				return Start(func() (r interface{}, err error) {
					r, err = function(v)
					return
				})
			}

		} else if function, status := callback.(func()); status {
			cs[i] = func(v interface{}) *Future {
				return Start(func() {
					function()
				})
			}

		} else if function, status := callback.(func() (r interface{}, err error)); status {
			cs[i] = func(v interface{}) *Future {
				return Start(func() (r interface{}, err error) {
					r, err = function()
					return
				})
			}

		} else {
			ok = false
			return
		}
	}

	for {
		v := future.loadVal()
		r := v.result
		if r != nil {
			result = future
			if r.Type == RESULT_SUCCESS && cs[0] != nil {
				result = cs[0](r.Result)
			} else if r.Type == RESULT_FAILURE && len(cs) > 1 && cs[1] != nil {
				result = cs[1](r.Result)
			}
		} else {
			newPipe := &pipe{}
			newPipe.pipeDoneTask = cs[0]
			if len(cs) > 1 {
				newPipe.pipeFailTask = cs[1]
			}
			newPipe.pipePromise = NewPromise()

			newVal := *v
			newVal.pipes = append(newVal.pipes, newPipe)

			// TODO: 使用 CAS 确保Future的state没有发生改变. 如果state发生改变, 将尝试CAS操作
			if atomic.CompareAndSwapPointer(&future.val, unsafe.Pointer(v), unsafe.Pointer(&newVal)) {
				result = newPipe.pipePromise.Future
				break
			}
		}
	}
	ok = true

	return
}

// 设置 Promise 最终的状态和结果, 只能被执行一次
func (future *Future) setResult(r *PromiseResult) (e error) { //r *PromiseResult) {
	defer func() {
		if err := getError(recover()); err != nil {
			e = err
			fmt.Println("\nerror in setResult():", err)
		}
	}()

	e = errors.New("cannot resolve/reject/cancel more than once")

	for {
		v := future.loadVal()
		if v.result != nil {
			return
		}
		newVal := *v
		newVal.result = r

		// todo: 使用 CAS 操作确保Promise的state没有发生改变
		// todo: 如果state发生, 必须获取最新的state并且尝试再次调用 CAS
		// todo: 原理方面的内容需要加深理解
		if atomic.CompareAndSwapPointer(&future.val, unsafe.Pointer(v), unsafe.Pointer(&newVal)) {
			// 关闭 final 确保 Get() 和 GetOrTimeout() 不再阻塞
			close(future.final)

			// call callback functions and start the Promise pipeline
			if len(v.dones) > 0 || len(v.fails) > 0 || len(v.always) > 0 || len(v.cancels) > 0 {
				go func() {
					execCallback(r, v.dones, v.fails, v.always, v.cancels)
				}()
			}

			// start the pipeline
			if len(v.pipes) > 0 {
				go func() {
					for _, pipe := range v.pipes {
						pipeTask, pipePromise := pipe.getPipe(r.Type == RESULT_SUCCESS)
						startPipe(r, pipeTask, pipePromise)
					}
				}()
			}
			e = nil
			break
		}
	}
	return
}

// 注册回调函数
func (future *Future) addCallback(callback interface{}, t callbackType) {
	if callback == nil {
		return
	}
	// 回调函数类型和回调函数要匹配
	if (t == CALLBACK_DONE) || (t == CALLBACK_FAIL) || (t == CALLBACK_ALWAYS) {
		if _, ok := callback.(func(v interface{})); !ok {
			panic(errors.New("callback function spec must be func(v interface{})"))
		}
	} else if t == CALLBACK_CANCEL {
		if _, ok := callback.(func()); !ok {
			panic(errors.New("callback function spec must be func()"))
		}
	}

	// 异步执行
	for {
		v := future.loadVal()
		r := v.result
		if r == nil {
			newVal := *v
			switch t {
			case CALLBACK_DONE:
				newVal.dones = append(newVal.dones, callback.(func(v interface{})))
			case CALLBACK_FAIL:
				newVal.fails = append(newVal.fails, callback.(func(v interface{})))
			case CALLBACK_ALWAYS:
				newVal.always = append(newVal.always, callback.(func(v interface{})))
			case CALLBACK_CANCEL:
				newVal.cancels = append(newVal.cancels, callback.(func()))
			}

			// 使用CAS确保Future的state未发生改变. 如果state发生改变, 会尝试CAS操作(函数返回的关键)
			if atomic.CompareAndSwapPointer(&future.val, unsafe.Pointer(v), unsafe.Pointer(&newVal)) {
				break
			}
		} else {
			// 执行回调函数
			if (t == CALLBACK_DONE && r.Type == RESULT_SUCCESS) ||
				(t == CALLBACK_FAIL && r.Type == RESULT_FAILURE) ||
				(t == CALLBACK_ALWAYS && r.Type != RESULT_CANCELLED) {
				callbackFunc := callback.(func(v interface{}))
				callbackFunc(r.Result)
			} else if t == CALLBACK_CANCEL && r.Type == RESULT_CANCELLED {
				callbackFunc := callback.(func())
				callbackFunc()
			}
			break
		}
	}
}
