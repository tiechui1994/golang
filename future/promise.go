package future

import (
	"unsafe"
	"math/rand"
)

var (
	CANCELLED error = &CancelledError{}
)

// future退出时的错误
type CancelledError struct{}

func (e *CancelledError) Error() string {
	return "Task be cancelled"
}

// Future最终的状态
type resultType int

const (
	RESULT_SUCCESS   resultType = iota
	RESULT_FAILURE
	RESULT_CANCELLED
)

// Promise的结果
// Type: 0, Result是Future的返回结果
// Type: 1, Result是Future的返回的错误
// Type: 2, Result是null
type PromiseResult struct {
	Result interface{}
	Type   resultType // success, failure, or cancelled?
}

/*********************************************************************
1. Promise提供一个对象作为结果的代理. 这个结果最初是未知的, 通常是因为其值尚未被计算出.
2. 可以使用Resolve() | Reject() | Cancel() 来设置Promise的最终结果.
3. Future只返回一个带有只读占位符视图.
*********************************************************************/
type Promise struct {
	*Future
}

/*********************************************************************

 方法总体说明:
	1. Cancel() Resolve() Reject(), 这些方法的调用会导致Promise任务执行完毕
	2. OnXxx() 此类型的方法是设置回调函数, 应当在Promise的任务执行完毕前调用添加

*********************************************************************/

// Cancel() 会将 Promise 的结果的 Type 设置为RESULT_CANCELLED。
// 如果promise被取消了, 调用Get()将返回nil和CANCELED错误. 并且所有的回调函数将不会被执行
func (promise *Promise) Cancel() (e error) {
	return promise.setResult(&PromiseResult{CANCELLED, RESULT_CANCELLED})
}

// Resolve() 会将 Promise 的结果的 Type 设置为RESULT_SUCCESS. Result设置为特定值
// 如果promise被取消了, 调用Get()将返回相应的值和nil
func (promise *Promise) Resolve(v interface{}) (e error) {
	return promise.setResult(&PromiseResult{v, RESULT_SUCCESS})
}

// Resolve() 会将 Promise 的结果的 Type 设置为RESULT_FAILURE.
func (promise *Promise) Reject(err error) (e error) {
	return promise.setResult(&PromiseResult{err, RESULT_FAILURE})
}

// OnSuccess注册一个回调函数, 该函数将在Promise有成功返回的时候调用. Promise的值将是Done回调函数的参数.
func (promise *Promise) OnSuccess(callback func(v interface{})) *Promise {
	promise.Future.OnSuccess(callback)
	return promise
}

// OnSuccess注册一个回调函数, 该函数将在Promise有失败返回的时候调用. Promise的error将是Done回调函数的参数.
func (promise *Promise) OnFailure(callback func(v interface{})) *Promise {
	promise.Future.OnFailure(callback)
	return promise
}

// OnComplete注册一个回调函数，该函数将在Promise成功或者失败返回的时候被调用.
// 根据Promise的状态，值或错误将是Always回调函数的参数.
// 如果Promise被调用, 则不会调用回调函数.
func (promise *Promise) OnComplete(callback func(v interface{})) *Promise {
	promise.Future.OnComplete(callback)
	return promise
}

// OnSuccess注册一个回调函数, 该函数将在Promise被取消的时候调用
func (promise *Promise) OnCancel(callback func()) *Promise {
	promise.Future.OnCancel(callback)
	return promise
}

func NewPromise() *Promise {
	value := &futureValue{
		dones:   make([]func(v interface{}), 0, 8),
		fails:   make([]func(v interface{}), 0, 8),
		always:  make([]func(v interface{}), 0, 4),
		cancels: make([]func(), 0, 2),
		pipes:   make([]*pipe, 0, 4),
		result:  nil,
	}

	promise := &Promise{
		Future: &Future{
			ID:    rand.Int(),
			final: make(chan struct{}),
			value: unsafe.Pointer(value),
		},
	}

	return promise
}
