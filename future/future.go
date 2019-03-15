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

//GetChan returns a channel than can be used to receive result of Promise
func (future *Future) GetChan() <-chan *PromiseResult {
	c := make(chan *PromiseResult, 1)
	future.OnComplete(func(v interface{}) {
		c <- future.loadResult()
	}).OnCancel(func() {
		c <- future.loadResult()
	})
	return c
}

//Get will block current goroutines until the Future is resolved/rejected/cancelled.
//If Future is resolved, value and nil will be returned
//If Future is rejected, nil and error will be returned.
//If Future is cancelled, nil and CANCELLED error will be returned.
func (future *Future) Get() (val interface{}, err error) {
	<-future.final
	return getFutureReturnVal(future.loadResult())
}

//GetOrTimeout is similar to Get(), but GetOrTimeout will not block after timeout.
//If GetOrTimeout returns with a timeout, timeout value will be true in return values.
//The unit of paramter is millisecond.
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

//Cancel sets the status of promise to RESULT_CANCELLED.
//If promise is cancelled, Get() will return nil and CANCELLED error.
//All callback functions will be not called if Promise is cancalled.
func (future *Future) Cancel() (e error) {
	return future.setResult(&PromiseResult{CANCELLED, RESULT_CANCELLED})
}

//OnSuccess registers a callback function that will be called when Promise is resolved.
//If promise is already resolved, the callback will immediately called.
//The value of Promise will be paramter of Done callback function.
func (future *Future) OnSuccess(callback func(v interface{})) *Future {
	future.addCallback(callback, CALLBACK_DONE)
	return future
}

//OnFailure registers a callback function that will be called when Promise is rejected.
//If promise is already rejected, the callback will immediately called.
//The error of Promise will be paramter of Fail callback function.
func (future *Future) OnFailure(callback func(v interface{})) *Future {
	future.addCallback(callback, CALLBACK_FAIL)
	return future
}

//OnComplete register a callback function that will be called when Promise is rejected or resolved.
//If promise is already rejected or resolved, the callback will immediately called.
//According to the status of Promise, value or error will be paramter of Always callback function.
//Value is the paramter if Promise is resolved, or error is the paramter if Promise is rejected.
//Always callback will be not called if Promise be called.
func (future *Future) OnComplete(callback func(v interface{})) *Future {
	future.addCallback(callback, CALLBACK_ALWAYS)
	return future
}

//OnCancel registers a callback function that will be called when Promise is cancelled.
//If promise is already cancelled, the callback will immediately called.
func (future *Future) OnCancel(callback func()) *Future {
	future.addCallback(callback, CALLBACK_CANCEL)
	return future
}

//Pipe registers one or two functions that returns a Future, and returns a proxy of pipeline Future.
//First function will be called when Future is resolved, the returned Future will be as pipeline Future.
//Secondary function will be called when Futrue is rejected, the returned Future will be as pipeline Future.
func (future *Future) Pipe(callbacks ...interface{}) (result *Future, ok bool) {
	if len(callbacks) == 0 ||
		(len(callbacks) == 1 && callbacks[0] == nil) ||
		(len(callbacks) > 1 && callbacks[0] == nil && callbacks[1] == nil) {
		result = future
		return
	}

	//ensure all callback functions match the spec "func(v interface{}) *Future"
	cs := make([]func(v interface{}) *Future, len(callbacks), len(callbacks))
	for i, callback := range callbacks {
		if c, ok1 := callback.(func(v interface{}) *Future); ok1 {
			cs[i] = c
		} else if c, ok1 := callback.(func() *Future); ok1 {
			cs[i] = func(v interface{}) *Future {
				return c()
			}
		} else if c, ok1 := callback.(func(v interface{})); ok1 {
			cs[i] = func(v interface{}) *Future {
				return Start(func() {
					c(v)
				})
			}
		} else if c, ok1 := callback.(func(v interface{}) (r interface{}, err error)); ok1 {
			cs[i] = func(v interface{}) *Future {
				return Start(func() (r interface{}, err error) {
					r, err = c(v)
					return
				})
			}
		} else if c, ok1 := callback.(func()); ok1 {
			cs[i] = func(v interface{}) *Future {
				return Start(func() {
					c()
				})
			}
		} else if c, ok1 := callback.(func() (r interface{}, err error)); ok1 {
			cs[i] = func(v interface{}) *Future {
				return Start(func() (r interface{}, err error) {
					r, err = c()
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

			//use CAS to ensure that the state of Future is not changed,
			//if the state is changed, will retry CAS operation.
			if atomic.CompareAndSwapPointer(&future.val, unsafe.Pointer(v), unsafe.Pointer(&newVal)) {
				result = newPipe.pipePromise.Future
				break
			}
		}
	}
	ok = true

	return
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

//setResult sets the value and final status of Promise, it will only be executed for once
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

		//Use CAS operation to ensure that the state of Promise isn't changed.
		//If the state is changed, must get latest state and try to call CAS again.
		//No ABA issue in future case because address of all objects are different.
		if atomic.CompareAndSwapPointer(&future.val, unsafe.Pointer(v), unsafe.Pointer(&newVal)) {
			//Close chEnd then all Get() and GetOrTimeout() will be unblocked
			close(future.final)

			//call callback functions and start the Promise pipeline
			if len(v.dones) > 0 || len(v.fails) > 0 || len(v.always) > 0 || len(v.cancels) > 0 {
				go func() {
					execCallback(r, v.dones, v.fails, v.always, v.cancels)
				}()
			}

			//start the pipeline
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

//handleOneCallback registers a callback function
func (future *Future) addCallback(callback interface{}, t callbackType) {
	if callback == nil {
		return
	}
	if (t == CALLBACK_DONE) ||
		(t == CALLBACK_FAIL) ||
		(t == CALLBACK_ALWAYS) {
		if _, ok := callback.(func(v interface{})); !ok {
			panic(errors.New("callback function spec must be func(v interface{})"))
		}
	} else if t == CALLBACK_CANCEL {
		if _, ok := callback.(func()); !ok {
			panic(errors.New("callback function spec must be func()"))
		}
	}

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

			//use CAS to ensure that the state of Future is not changed,
			//if the state is changed, will retry CAS operation.
			if atomic.CompareAndSwapPointer(&future.val, unsafe.Pointer(v), unsafe.Pointer(&newVal)) {
				break
			}
		} else {
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
