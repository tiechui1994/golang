package future

import (
	"sync/atomic"
)

type anyPromiseResult struct {
	result interface{}
	i      int
}

/**************************************************************
开启一个goroutine用来执行一个act函数并返回一个Future(act执行的结果).
如果option参数是true, act函数会被异步调用.

act的函数类型可以是以下4种:
  func() (r interface{}, err error)

  func()

  func(c promise.Canceller) (r interface{}, err error)
     c可以调用c.IsCancelled()方法退出函数执行

  func(promise.Canceller)
***************************************************************/
func Start(action interface{}, syncs ...bool) *Future {
	promise := NewPromise()
	if f, ok := action.(*Future); ok {
		return f
	}

	if proxy := getAction(promise, action); proxy != nil {
		if syncs != nil && len(syncs) > 0 && !syncs[0] {
			// 同步调用
			result, err := proxy()
			if promise.IsCancelled() {
				promise.Cancel()
			} else {
				if err == nil {
					promise.Resolve(result)
				} else {
					promise.Reject(err)
				}
			}
		} else {
			// 异步调用
			go func() {
				r, err := proxy()
				if promise.IsCancelled() {
					promise.Cancel()
				} else {
					if err == nil {
						promise.Resolve(r)
					} else {
						promise.Reject(err)
					}
				}
			}()
		}
	}

	return promise.Future
}

// 包装Future
func Wrap(value interface{}) *Future {
	promise := NewPromise()
	if e, ok := value.(error); !ok {
		promise.Resolve(value)
	} else {
		promise.Reject(e)
	}

	return promise.Future
}

// 返回一个Future
// 如果任何一个Future执行成功, 当前的Future也将会执行成功,并且返回已经成功执行的Future的值; 否则,
// 当前的Future将会执行失败, 并且返回所有Future的执行结果.
func WhenAny(actions ...interface{}) *Future {
	return WhenAnyMatched(nil, actions...)
}

// 返回一个Future
// 如果任何一个Future执行成功并且predicate()函数执返回true, 当前的Future也将会执行成功,并且返回已经成功执行的Future的值.
// 如果所有的Future都被取消, 当前的Future也会被取消; 否则, 当前的Future将会执行失败NoMatchedError, 并且返回所有Future的执行结果.
func WhenAnyMatched(predicate func(interface{}) bool, actions ...interface{}) *Future {
	if predicate == nil {
		predicate = func(v interface{}) bool { return true }
	}

	// todo: action包装成Future
	functions := make([]*Future, len(actions))
	for i, act := range actions {
		functions[i] = Start(act)
	}

	// todo: 构建 Promise 和 返回结果集合
	promise, results := NewPromise(), make([]interface{}, len(functions))
	if len(actions) == 0 {
		promise.Resolve(nil)
	}

	// todo: 设置channel
	chFails, chDones := make(chan anyPromiseResult), make(chan anyPromiseResult)
	go func() {
		for i, function := range functions {
			k := i
			function.OnSuccess(func(v interface{}) {
				defer func() { _ = recover() }()
				chDones <- anyPromiseResult{v, k}
			}).OnFailure(func(v interface{}) {
				defer func() { _ = recover() }()
				chFails <- anyPromiseResult{v, k}
			}).OnCancel(func() {
				defer func() { _ = recover() }()
				chFails <- anyPromiseResult{CANCELLED, k}
			})
		}
	}()

	// todo: 根据预定的规则执行(阻塞)
	if len(functions) == 1 {
		select {
		case fail := <-chFails:
			if _, ok := fail.result.(CancelledError); ok {
				promise.Cancel()
			} else {
				promise.Reject(newNoMatchedError1(fail.result))
			}
		case done := <-chDones:
			if predicate(done.result) {
				promise.Resolve(done.result)
			} else {
				promise.Reject(newNoMatchedError1(done.result))
			}
		}
	} else {
		go func() {
			defer func() {
				if e := recover(); e != nil {
					promise.Reject(newErrorWithStacks(e))
				}
			}()

			j := 0
			for {
				// todo: 有一个执行结果返回
				select {
				case fail := <-chFails:
					results[fail.i] = getError(fail.result)
				case done := <-chDones:
					if predicate(done.result) {
						// 任何一个Future成功返回, 当前的Future也需要成功返回. 此时需要取消其他的Future的执行
						for _, function := range functions {
							function.Cancel()
						}

						// 关闭channel以避免 `发送方` 被阻塞
						closeChan := func(c chan anyPromiseResult) {
							defer func() { _ = recover() }()
							close(c)
						}
						closeChan(chDones)
						closeChan(chFails)

						// 成功执行并且返回
						promise.Resolve(done.result) // 成功执行并返回
						return
					} else {
						results[done.i] = done.result
					}
				}

				// todo: 执行的次数和functions的长度一致, 需要退出循环
				if j++; j == len(functions) {
					m := 0
					for _, result := range results {
						switch val := result.(type) {
						case CancelledError:
						default:
							m++
							_ = val
						}
					}
					if m > 0 {
						promise.Reject(newNoMatchedError(results)) // 存在取消的Future
					} else {
						promise.Cancel() // 所有的Future都已经被执行(没有取消), 这个时候可以取消当前Promise的执行
					}
					break
				}
			}
		}()
	}

	return promise.Future
}

// 返回一个Future
// 如果所有的Future都成功执行, 当前的Future也会成功执行并且返回相应的结果数组(成功执行的Future的结果);
// 否则, 当前的Future将会执行失败, 并且返回所有Future的执行结果.
func WhenAll(actions ...interface{}) (fu *Future) {
	pr := NewPromise()
	fu = pr.Future

	if len(actions) == 0 {
		pr.Resolve([]interface{}{})
		return
	}

	fs := make([]*Future, len(actions))
	for i, act := range actions {
		fs[i] = Start(act)
	}
	fu = whenAllFuture(fs...)
	return
}

// 返回一个Future
// 如果所有的Future都成功执行, 当前的Future也会成功执行并且返回相应的结果数组(成功执行的Future的结果).
// 如果任何一个Future都被取消, 当前的Future也会被取消; 否则, 当前的Future将会执行失败, 并且返回所有Future的执行结果.
func whenAllFuture(fs ...*Future) *Future {
	wf := NewPromise()
	rs := make([]interface{}, len(fs))

	if len(fs) == 0 {
		wf.Resolve([]interface{}{})
	} else {
		n := int32(len(fs))
		cancelOthers := func(j int) {
			for k, f1 := range fs {
				if k != j {
					f1.Cancel()
				}
			}
		}

		go func() {
			isCancelled := int32(0)
			for i, f := range fs {
				j := i

				f.OnSuccess(func(v interface{}) {
					rs[j] = v
					if atomic.AddInt32(&n, -1) == 0 {
						wf.Resolve(rs)
					}
				}).OnFailure(func(v interface{}) {
					if atomic.CompareAndSwapInt32(&isCancelled, 0, 1) {
						//try to cancel all futures
						cancelOthers(j)

						//errs := make([]error, 0, 1)
						//errs = append(errs, v.(error))
						e := newAggregateError1("Error appears in WhenAll:", v)
						wf.Reject(e)
					}
				}).OnCancel(func() {
					if atomic.CompareAndSwapInt32(&isCancelled, 0, 1) {
						//try to cancel all futures
						cancelOthers(j)

						wf.Cancel()
					}
				})
			}
		}()
	}

	return wf.Future
}
