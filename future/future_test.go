package future

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/smartystreets/goconvey/convey"
)

const (
	TASK_END      = "task be end,"
	CALL_DONE     = "callback done,"
	CALL_FAIL     = "callback fail,"
	CALL_ALWAYS   = "callback always,"
	WAIT_TASK     = "wait task end,"
	GET           = "get task result,"
	DONE_Pipe_END = "task Pipe done be end,"
	FAIL_Pipe_END = "task Pipe fail be end,"
)

// errorLinq is a trivial implementation of error.
type myError struct {
	val interface{}
}

func (e *myError) Error() string {
	return fmt.Sprintf("%v", e.val)
}

func newMyError(v interface{}) *myError {
	return &myError{v}
}

// Promise最简单的使用 Resolve | Reject
func TestResolveAndReject(t *testing.T) {
	convey.Convey("When Promise is resolved", t, func() {
		p := NewPromise()
		go func() {
			time.Sleep(50 * time.Millisecond)
			p.Resolve("ok")
		}()

		convey.Convey("Should return the argument of Resolve", func() {
			val, err := p.Get()
			convey.So(val, convey.ShouldEqual, "ok")
			convey.So(err, convey.ShouldBeNil)
		})
	})

	convey.Convey("When Promise is rejected", t, func() {
		p := NewPromise()
		go func() {
			time.Sleep(50 * time.Millisecond)
			p.Reject(errors.New("fail"))
		}()

		convey.Convey("Should return error", func() {
			val, err := p.Get()
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(val, convey.ShouldEqual, nil)
		})
	})
}

// Promise最简单的使用 Cancel
func TestCancel(t *testing.T) {
	convey.Convey("When Promise is cancelled", t, func() {
		p := NewPromise()
		go func() {
			time.Sleep(50 * time.Millisecond)
			p.Cancel()
		}()

		convey.Convey("Should return CancelledError", func() {
			val, err := p.Get()
			convey.So(val, convey.ShouldBeNil)
			convey.So(err, convey.ShouldEqual, CANCELLED)
			convey.So(p.IsCancelled(), convey.ShouldBeTrue)
		})
	})
}

// Promise 超时机制测试
func TestGetOrTimeout(t *testing.T) {
	timout := 50 * time.Millisecond
	convey.Convey("When Promise is unfinished", t, func() {
		p := NewPromise()

		go func() {
			<-time.After(timout)
			p.Resolve("ok") // 成功
		}()

		convey.Convey("Timeout should be true", func() {
			val, err, timeout := p.GetOrTimeout(10)
			convey.So(val, convey.ShouldBeNil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(timeout, convey.ShouldBeTrue)
		})

		convey.Convey("When Promise is resolved, the argument of Resolve should be returned", func() {
			val, err, timeout := p.GetOrTimeout(51) // 需要比timeout大一点
			convey.So(val, convey.ShouldEqual, "ok")
			convey.So(err, convey.ShouldBeNil)
			convey.So(timeout, convey.ShouldBeFalse)
		})
	})

	convey.Convey("When Promise is rejected", t, func() {
		p := NewPromise()
		go func() {
			<-time.After(timout)
			p.Reject(errors.New("fail")) // 失败
		}()

		convey.Convey("Should return nil", func() {
			val, err, timeout := p.GetOrTimeout(10)
			convey.So(val, convey.ShouldBeNil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(timeout, convey.ShouldBeTrue)
		})

		convey.Convey("Should return error", func() {
			val, err, timeout := p.GetOrTimeout(51)
			convey.So(val, convey.ShouldBeNil)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(timeout, convey.ShouldBeFalse)
		})
	})

	convey.Convey("When Promise is cancelled", t, func() {
		p := NewPromise()
		go func() {
			<-time.After(timout)
			p.Cancel() // 取消
		}()

		convey.Convey("Should return nil", func() {
			val, err, timeout := p.GetOrTimeout(10)
			convey.So(val, convey.ShouldBeNil)
			convey.So(err, convey.ShouldBeNil)
			convey.So(timeout, convey.ShouldBeTrue)

			convey.So(p.IsCancelled(), convey.ShouldBeFalse)
		})

		convey.Convey("Should return CancelledError", func() {
			val, err, timeout := p.GetOrTimeout(51)
			convey.So(val, convey.ShouldBeNil)
			convey.So(err, convey.ShouldEqual, CANCELLED)
			convey.So(timeout, convey.ShouldBeFalse)

			convey.So(p.IsCancelled(), convey.ShouldBeTrue)
		})
	})
}

func TestGetChan(t *testing.T) {
	timout := 50 * time.Millisecond
	convey.Convey("When Promise is resolved", t, func() {
		p := NewPromise()
		go func() {
			<-time.After(timout)
			p.Resolve("ok")
		}()
		convey.Convey("Should receive the argument of Resolve from returned channel", func() {
			fr, ok := <-p.GetChan()
			convey.So(fr.Result, convey.ShouldEqual, "ok")
			convey.So(fr.Type, convey.ShouldEqual, RESULT_SUCCESS)
			convey.So(ok, convey.ShouldBeTrue)
		})
	})

	convey.Convey("When Promise is rejected", t, func() {
		p := NewPromise()
		go func() {
			<-time.After(timout)
			p.Reject(errors.New("fail"))
		}()
		convey.Convey("Should receive error from returned channel", func() {
			fr, ok := <-p.GetChan()
			convey.So(fr.Result, convey.ShouldNotBeNil)
			convey.So(fr.Type, convey.ShouldEqual, RESULT_FAILURE)
			convey.So(ok, convey.ShouldBeTrue)
		})
	})

	convey.Convey("When Promise is cancelled", t, func() {
		p := NewPromise()
		go func() {
			<-time.After(timout)
			p.Cancel()
		}()
		convey.Convey("Should receive CancelledError from returned channel", func() {
			fr, ok := <-p.GetChan()
			convey.So(fr.Result, convey.ShouldEqual, CANCELLED)
			convey.So(p.IsCancelled(), convey.ShouldBeTrue)
			convey.So(fr.Type, convey.ShouldEqual, RESULT_CANCELLED)
			convey.So(ok, convey.ShouldBeTrue)
		})

		convey.Convey("Should receive CancelledError from returned channel at second time", func() {
			fr, ok := <-p.GetChan()
			convey.So(fr.Result, convey.ShouldEqual, CANCELLED)
			convey.So(p.IsCancelled(), convey.ShouldBeTrue)
			convey.So(fr.Type, convey.ShouldEqual, RESULT_CANCELLED)
			convey.So(ok, convey.ShouldBeTrue)
		})
	})
}

func TestFuture(t *testing.T) {
	convey.Convey("Future can receive return value and status but cannot change the status", t, func() {
		var fu *Future
		convey.Convey("When Future is resolved", func() {
			func() {
				p := NewPromise()
				go func() {
					time.Sleep(50 * time.Millisecond)
					p.Resolve("ok")
				}()
				fu = p.Future
			}()
			r, err := fu.Get()
			convey.So(r, convey.ShouldEqual, "ok")
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("When Future is rejected", func() {
			func() {
				p := NewPromise()
				go func() {
					time.Sleep(50 * time.Millisecond)
					p.Reject(errors.New("fail"))
				}()
				fu = p.Future
			}()
			r, err := fu.Get()
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(r, convey.ShouldEqual, nil)
		})

		convey.Convey("When Future is cancelled", func() {
			func() {
				p := NewPromise()
				go func() {
					time.Sleep(50 * time.Millisecond)
					p.Cancel()
				}()
				fu = p.Future
			}()
			r, err := fu.Get()
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(r, convey.ShouldEqual, nil)
		})
	})

}

func TestCallbacks(t *testing.T) {
	timout := 50 * time.Millisecond
	done, always, fail, cancel := false, false, false, false

	p := NewPromise()
	go func() {
		<-time.After(timout)
		p.Resolve("ok")
	}()

	convey.Convey("When Promise is resolved", t, func() {
		p.OnSuccess(func(v interface{}) {
			done = true
			convey.Convey("The argument of Done should be 'ok'", t, func() {
				convey.So(v, convey.ShouldEqual, "ok")
			})
		}).OnComplete(func(v interface{}) {
			always = true
			convey.Convey("The argument of Always should be 'ok'", t, func() {
				convey.So(v, convey.ShouldEqual, "ok")
			})
		}).OnFailure(func(v interface{}) {
			fail = true
			panic("Unexpected calling")
		})
		r, err := p.Get()

		//The code after Get() and the callback will be concurrent run
		//So sleep 52 ms to wait all callback be done
		time.Sleep(52 * time.Millisecond)

		convey.Convey("Should call the Done and Always callbacks", func() {
			convey.So(r, convey.ShouldEqual, "ok")
			convey.So(err, convey.ShouldBeNil)
			convey.So(done, convey.ShouldEqual, true)
			convey.So(always, convey.ShouldEqual, true)
			convey.So(fail, convey.ShouldEqual, false)
		})
	})

	convey.Convey("When adding the callback after Promise is resolved", t, func() {
		done, always, fail := false, false, false
		p.OnSuccess(func(v interface{}) {
			done = true
			convey.Convey("The argument of Done should be 'ok'", func() {
				convey.So(v, convey.ShouldEqual, "ok")
			})
		}).OnComplete(func(v interface{}) {
			always = true
			convey.Convey("The argument of Always should be 'ok'", func() {
				convey.So(v, convey.ShouldEqual, "ok")
			})
		}).OnFailure(func(v interface{}) {
			fail = true
			panic("Unexpected calling")
		})
		convey.Convey("Should immediately run the Done and Always callbacks", func() {
			convey.So(done, convey.ShouldEqual, true)
			convey.So(always, convey.ShouldEqual, true)
			convey.So(fail, convey.ShouldEqual, false)
		})
	})

	var e *error = nil
	done, always, fail = false, false, false
	p = NewPromise()
	go func() {
		<-time.After(timout)
		p.Reject(errors.New("fail"))
	}()

	convey.Convey("When Promise is rejected", t, func() {
		p.OnSuccess(func(v interface{}) {
			done = true
			panic("Unexpected calling")
		}).OnComplete(func(v interface{}) {
			always = true
			convey.Convey("The argument of Always should be error", t, func() {
				convey.So(v, convey.ShouldImplement, e)
			})
		}).OnFailure(func(v interface{}) {
			fail = true
			convey.Convey("The argument of Fail should be error", t, func() {
				convey.So(v, convey.ShouldImplement, e)
			})
		})
		r, err := p.Get()

		time.Sleep(52 * time.Millisecond)

		convey.Convey("Should call the Fail and Always callbacks", func() {
			convey.So(r, convey.ShouldEqual, nil)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(done, convey.ShouldEqual, false)
			convey.So(always, convey.ShouldEqual, true)
			convey.So(fail, convey.ShouldEqual, true)
		})
	})

	convey.Convey("When adding the callback after Promise is rejected", t, func() {
		done, always, fail = false, false, false
		p.OnSuccess(func(v interface{}) {
			done = true
			panic("Unexpected calling")
		}).OnComplete(func(v interface{}) {
			always = true
			convey.Convey("The argument of Always should be error", func() {
				convey.So(v, convey.ShouldImplement, e)
			})
		}).OnFailure(func(v interface{}) {
			fail = true
			convey.Convey("The argument of Fail should be error", func() {
				convey.So(v, convey.ShouldImplement, e)
			})
		})
		convey.Convey("Should immediately run the Fail and Always callbacks", func() {
			convey.So(done, convey.ShouldEqual, false)
			convey.So(always, convey.ShouldEqual, true)
			convey.So(fail, convey.ShouldEqual, true)
		})
	})

	done, always, fail = false, false, false
	p = NewPromise()
	go func() {
		<-time.After(timout)
		p.Cancel()
	}()

	convey.Convey("When Promise is cancelled", t, func() {
		done, always, fail, cancel = false, false, false, false
		p.OnSuccess(func(v interface{}) {
			done = true
		}).OnComplete(func(v interface{}) {
			always = true
		}).OnFailure(func(v interface{}) {
			fail = true
		}).OnCancel(func() {
			cancel = true
		})
		r, err := p.Get()

		time.Sleep(62 * time.Millisecond)

		convey.Convey("Only cancel callback be called", func() {
			convey.So(r, convey.ShouldBeNil)
			convey.So(err, convey.ShouldNotBeNil)
			convey.So(done, convey.ShouldEqual, false)
			convey.So(always, convey.ShouldEqual, false)
			convey.So(fail, convey.ShouldEqual, false)
			convey.So(cancel, convey.ShouldEqual, true)
		})
	})

	convey.Convey("When adding the callback after Promise is cancelled", t, func() {
		done, always, fail, cancel = false, false, false, false
		p.OnSuccess(func(v interface{}) {
			done = true
		}).OnComplete(func(v interface{}) {
			always = true
		}).OnFailure(func(v interface{}) {
			fail = true
		}).OnCancel(func() {
			cancel = true
		})
		convey.Convey("Should not call any callbacks", func() {
			convey.So(done, convey.ShouldEqual, false)
			convey.So(always, convey.ShouldEqual, false)
			convey.So(fail, convey.ShouldEqual, false)
			convey.So(cancel, convey.ShouldEqual, true)
		})
	})

}

func TestStart(t *testing.T) {

	convey.Convey("Test start func()", t, func() {
		convey.Convey("When task completed", func() {
			f := Start(func() {})
			r, err := f.Get()
			convey.So(r, convey.ShouldBeNil)
			convey.So(err, convey.ShouldBeNil)
		})
		convey.Convey("When task panic error", func() {
			f := Start(func() { panic("fail") })
			r, err := f.Get()
			convey.So(r, convey.ShouldBeNil)
			convey.So(err, convey.ShouldNotBeNil)
		})
	})

	convey.Convey("Test start func()(interface{}, error)", t, func() {
		convey.Convey("When task completed", func() {
			f := Start(func() (interface{}, error) {
				time.Sleep(10)
				return "ok", nil
			})
			r, err := f.Get()
			convey.So(r, convey.ShouldEqual, "ok")
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("When task returned error", func() {
			f := Start(func() (interface{}, error) {
				time.Sleep(10)
				return "fail", errors.New("fail")
			})
			r, err := f.Get()
			convey.So(r, convey.ShouldBeNil)
			convey.So(err, convey.ShouldNotBeNil)
		})

		convey.Convey("When task panic error", func() {
			f := Start(func() (interface{}, error) { panic("fail") })
			r, err := f.Get()
			convey.So(r, convey.ShouldBeNil)
			convey.So(err, convey.ShouldNotBeNil)
		})
	})

	convey.Convey("Test start func(canceller Canceller)", t, func() {
		convey.Convey("When task completed", func() {
			f := Start(func(canceller Canceller) {
				time.Sleep(10)
			})
			r, err := f.Get()
			convey.So(r, convey.ShouldBeNil)
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("When task be cancelled", func() {
			f := Start(func(canceller Canceller) {
				time.Sleep(10)
				if canceller.IsCancelled() {
					return
				}
			})
			f.Cancel()
			r, err := f.Get()
			convey.So(f.IsCancelled(), convey.ShouldBeTrue)
			convey.So(r, convey.ShouldBeNil)
			convey.So(err, convey.ShouldEqual, CANCELLED)
			convey.So(f.IsCancelled(), convey.ShouldBeTrue)
		})
		convey.Convey("When task panic error", func() {
			f := Start(func(canceller Canceller) { panic("fail") })
			r, err := f.Get()
			convey.So(r, convey.ShouldBeNil)
			convey.So(err, convey.ShouldNotBeNil)
		})
	})

	convey.Convey("Test start func(canceller Canceller)(interface{}, error)", t, func() {
		convey.Convey("When task be cancenlled", func() {
			task := func(canceller Canceller) (interface{}, error) {
				i := 0
				for i < 50 {
					if canceller.IsCancelled() {
						return nil, nil
					}
					time.Sleep(100 * time.Millisecond)
				}
				panic("exception")
			}

			f := Start(task)
			f.Cancel()
			r, err := f.Get()

			convey.So(f.IsCancelled(), convey.ShouldBeTrue)
			convey.So(r, convey.ShouldBeNil)
			convey.So(err, convey.ShouldEqual, CANCELLED)
			convey.So(f.IsCancelled(), convey.ShouldBeTrue)
		})

		convey.Convey("When task panic error", func() {
			f := Start(func(canceller Canceller) (interface{}, error) {
				panic("fail")
			})
			r, err := f.Get()
			convey.So(r, convey.ShouldBeNil)
			convey.So(err, convey.ShouldNotBeNil)
		})
	})

}

func TestPipe(t *testing.T) {
	timout := 50 * time.Millisecond
	taskDonePipe := func(v interface{}) *Future {
		return Start(func() (interface{}, error) {
			<-time.After(timout)
			return v.(string) + "2", nil
		})
	}

	taskFailPipe := func() (interface{}, error) {
		<-time.After(timout)
		return "fail2", nil
	}

	convey.Convey("When task completed", t, func() {
		p := NewPromise()
		go func() {
			<-time.After(timout)
			p.Resolve("ok")
		}()
		fu, ok := p.Pipe(taskDonePipe, taskFailPipe)
		r, err := fu.Get()
		convey.Convey("the done callback will be called, the future returned by done callback will be returned as chain future", func() {
			convey.So(r, convey.ShouldEqual, "ok2")
			convey.So(err, convey.ShouldBeNil)
			convey.So(ok, convey.ShouldEqual, true)
		})
	})

	convey.Convey("When task failed", t, func() {
		p := NewPromise()
		go func() {
			<-time.After(timout)
			p.Reject(errors.New("fail"))
		}()
		fu, ok := p.Pipe(taskDonePipe, taskFailPipe)
		r, err := fu.Get()

		convey.Convey("the fail callback will be called, the future returned by fail callback will be returned as chain future", func() {
			convey.So(r, convey.ShouldEqual, "fail2")
			convey.So(err, convey.ShouldBeNil)
			convey.So(ok, convey.ShouldEqual, true)
		})
	})

	convey.Convey("Test pipe twice", t, func() {
		p := NewPromise()
		pipeFuture1, ok1 := p.Pipe(taskDonePipe, taskFailPipe)
		convey.Convey("Calling Pipe succeed at first time", func() {
			convey.So(ok1, convey.ShouldEqual, true)
		})
		pipeFuture2, ok2 := p.Pipe(taskDonePipe, taskFailPipe)
		convey.Convey("Calling Pipe succeed at second time", func() {
			convey.So(ok2, convey.ShouldEqual, true)
		})
		p.Resolve("ok")

		r, _ := pipeFuture1.Get()
		convey.Convey("Pipeline future 1 should return ok2", func() {
			convey.So(r, convey.ShouldEqual, "ok2")
		})

		r2, _ := pipeFuture2.Get()
		convey.Convey("Pipeline future 2 should return ok2", func() {
			convey.So(r2, convey.ShouldEqual, "ok2")
		})
	})
}

func TestWhenAny(t *testing.T) {
	convey.Convey("Test WhenAny", t, func() {
		whenAnyTasks := func(t1 int, t2 int) *Future {
			timeouts := []time.Duration{time.Duration(t1), time.Duration(t2)}
			getTask := func(i int) func() (interface{}, error) {
				return func() (interface{}, error) {
					if timeouts[i] > 0 {
						time.Sleep(timeouts[i] * time.Millisecond)
						return "ok" + strconv.Itoa(i), nil
					} else {
						time.Sleep((-1 * timeouts[i]) * time.Millisecond)
						return nil, newMyError("fail" + strconv.Itoa(i))
					}
				}
			}
			task0 := getTask(0)
			task1 := getTask(1)
			f := WhenAny(task0, task1)
			return f
		}

		convey.Convey("When all tasks completed, and task 1 be first to complete", func() {
			r, err := whenAnyTasks(200, 250).Get()
			convey.So(r, convey.ShouldEqual, "ok0")
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("When all tasks completed, and task 2 be first to complete", func() {
			r, err := whenAnyTasks(280, 250).Get()
			convey.So(r, convey.ShouldEqual, "ok1")
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("When all tasks failed", func() {
			r, err := whenAnyTasks(-280, -250).Get()
			errs := err.(*NoMatchedError).Results
			convey.So(r, convey.ShouldBeNil)
			convey.So(errs[0].(*myError).val, convey.ShouldEqual, "fail0")
			convey.So(errs[1].(*myError).val, convey.ShouldEqual, "fail1")
		})

		convey.Convey("When one task completed", func() {
			r, err := whenAnyTasks(-280, 150).Get()
			convey.So(r, convey.ShouldEqual, "ok1")
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("When no task be passed", func() {
			r, err := WhenAny().Get()
			convey.So(r, convey.ShouldBeNil)
			convey.So(err, convey.ShouldBeNil)
		})
	})

	convey.Convey("Test WhenAny, and task can be cancelled", t, func() {
		var c1, c2 bool
		whenAnyCanCancelTasks := func(t1 int, t2 int) *Future {
			timeouts := []time.Duration{time.Duration(t1), time.Duration(t2)}
			getTask := func(i int) func(canceller Canceller) (interface{}, error) {
				return func(canceller Canceller) (interface{}, error) {
					for j := 0; j < 10; j++ {
						if timeouts[i] > 0 {
							time.Sleep(timeouts[i] * time.Millisecond)
						} else {
							time.Sleep((-1 * timeouts[i]) * time.Millisecond)
						}
						if canceller.IsCancelled() {
							if i == 0 {
								c1 = true
							} else {
								c2 = true
							}
							return nil, nil
						}
					}
					if timeouts[i] > 0 {
						return "ok" + strconv.Itoa(i), nil
					} else {
						return nil, newMyError("fail" + strconv.Itoa(i))
					}
				}
			}
			task0 := getTask(0)
			task1 := getTask(1)
			f := WhenAny(Start(task0), Start(task1))
			return f
		}
		convey.Convey("When task 1 is the first to complete, task 2 will be cancelled", func() {
			r, err := whenAnyCanCancelTasks(10, 250).Get()

			convey.So(r, convey.ShouldEqual, "ok0")
			convey.So(err, convey.ShouldBeNil)
			time.Sleep(1000 * time.Millisecond)
			convey.So(c2, convey.ShouldEqual, true)
		})

		convey.Convey("When task 2 is the first to complete, task 1 will be cancelled", func() {
			r, err := whenAnyCanCancelTasks(200, 10).Get()

			convey.So(r, convey.ShouldEqual, "ok1")
			convey.So(err, convey.ShouldBeNil)
			time.Sleep(1000 * time.Millisecond)
			convey.So(c1, convey.ShouldEqual, true)
		})

	})
}

func TestWhenAnyTrue(t *testing.T) {
	c1, c2 := false, false
	startTwoCanCancelTask := func(t1 int, t2 int, predicate func(interface{}) bool) *Future {
		timeouts := []time.Duration{time.Duration(t1), time.Duration(t2)}
		getTask := func(i int) func(canceller Canceller) (interface{}, error) {
			return func(canceller Canceller) (interface{}, error) {
				for j := 0; j < 10; j++ {
					if timeouts[i] > 0 {
						time.Sleep(timeouts[i] * time.Millisecond)
					} else {
						time.Sleep((-1 * timeouts[i]) * time.Millisecond)
					}
					if canceller.IsCancelled() {
						if i == 0 {
							c1 = true
						} else {
							c2 = true
						}
						return nil, nil
					}
				}
				if timeouts[i] > 0 {
					return "ok" + strconv.Itoa(i), nil
				} else {
					return nil, newMyError("fail" + strconv.Itoa(i))
				}
			}
		}
		task0 := getTask(0)
		task1 := getTask(1)
		f := WhenAnyMatched(predicate, Start(task0), Start(task1))
		return f
	}
	//第一个任务先完成，第二个后完成，并且设定条件为返回值==第一个的返回值
	convey.Convey("When the task1 is the first to complete, and predicate returns true", t, func() {
		r, err := startTwoCanCancelTask(30, 250, func(v interface{}) bool {
			return v.(string) == "ok0"
		}).Get()
		convey.So(r, convey.ShouldEqual, "ok0")
		convey.So(err, convey.ShouldBeNil)
		time.Sleep(1000 * time.Millisecond)
		convey.So(c2, convey.ShouldEqual, true)
	})

	//第一个任务后完成，第二个先完成，并且设定条件为返回值==第二个的返回值
	convey.Convey("When the task2 is the first to complete, and predicate returns true", t, func() {
		c1, c2 = false, false
		r, err := startTwoCanCancelTask(230, 50, func(v interface{}) bool {
			return v.(string) == "ok1"
		}).Get()
		convey.So(r, convey.ShouldEqual, "ok1")
		convey.So(err, convey.ShouldBeNil)
		time.Sleep(1000 * time.Millisecond)
		convey.So(c1, convey.ShouldEqual, true)
	})

	//第一个任务后完成，第二个先完成，并且设定条件为返回值不等于任意一个任务的返回值
	convey.Convey("When the task2 is the first to complete, and predicate always returns false", t, func() {
		c1, c2 = false, false
		r, err := startTwoCanCancelTask(30, 250, func(v interface{}) bool {
			return v.(string) == "ok11"
		}).Get()

		_, ok := err.(*NoMatchedError)
		convey.So(r, convey.ShouldBeNil)
		convey.So(ok, convey.ShouldBeTrue)
		convey.So(err, convey.ShouldNotBeNil)

		time.Sleep(1000 * time.Millisecond)
		convey.So(c1, convey.ShouldEqual, false)
		convey.So(c2, convey.ShouldEqual, false)
	})

	//convey.Convey("When all tasks be cancelled", t, func() {
	//	getTask := func(canceller Canceller) (interface{}, error) {
	//		for {
	//			time.Sleep(50 * time.Millisecond)
	//			if canceller.IsCancellationRequested() {
	//				canceller.Cancel()
	//				return nil, nil
	//			}
	//		}
	//	}

	//	f1 := Start(getTask)
	//	f2 := Start(getTask)
	//	f3 := WhenAnyMatched(nil, f1, f2)

	//	f1.RequestCancel()
	//	f2.RequestCancel()

	//	r, _ := f3.Get()
	//	convey.So(r, convey.ShouldBeNil)
	//})

}

func TestWhenAll(t *testing.T) {
	startTwoTask := func(t1 int, t2 int) (f *Future) {
		timeouts := []time.Duration{time.Duration(t1), time.Duration(t2)}
		getTask := func(i int) func() (interface{}, error) {
			return func() (interface{}, error) {
				if timeouts[i] > 0 {
					time.Sleep(timeouts[i] * time.Millisecond)
					return "ok" + strconv.Itoa(i), nil
				} else {
					time.Sleep((-1 * timeouts[i]) * time.Millisecond)
					return nil, newMyError("fail" + strconv.Itoa(i))
				}
			}
		}
		task0 := getTask(0)
		task1 := getTask(1)
		f = WhenAll(task0, task1)
		return f
	}
	convey.Convey("Test WhenAllFuture", t, func() {
		whenTwoTask := func(t1 int, t2 int) *Future {
			return startTwoTask(t1, t2)
		}
		convey.Convey("When all tasks completed, and the task1 is the first to complete", func() {
			r, err := whenTwoTask(200, 230).Get()
			convey.So(r, shouldSlicesReSame, []interface{}{"ok0", "ok1"})
			convey.So(err, convey.ShouldBeNil)
		})

		//convey.Convey("When all tasks completed, and the task1 is the first to complete", func() {
		//	r, err := whenTwoTask(230, 200).Get()
		//	convey.So(r, shouldSlicesReSame, []interface{}{"ok0", "ok1"})
		//	convey.So(err, convey.ShouldBeNil)
		//})

		convey.Convey("When task1 failed, but task2 is completed", func() {
			r, err := whenTwoTask(-250, 210).Get()
			convey.So(err.(*AggregateError).InnerErrs[0].(*myError).val, convey.ShouldEqual, "fail0")
			convey.So(r, convey.ShouldBeNil)
		})

		convey.Convey("When all tasks failed", func() {
			r, err := whenTwoTask(-250, -110).Get()
			convey.So(err.(*AggregateError).InnerErrs[0].(*myError).val, convey.ShouldEqual, "fail1")
			convey.So(r, convey.ShouldBeNil)
		})

		convey.Convey("When no task be passed", func() {
			r, err := whenAllFuture().Get()
			convey.So(r, shouldSlicesReSame, []interface{}{})
			convey.So(err, convey.ShouldBeNil)
		})

		convey.Convey("When all tasks be cancelled", func() {
			getTask := func(canceller Canceller) (interface{}, error) {
				for {
					time.Sleep(50 * time.Millisecond)
					if canceller.IsCancelled() {
						return nil, nil
					}
				}
			}

			f1 := Start(getTask)
			f2 := Start(getTask)
			f3 := WhenAll(f1, f2)

			f1.Cancel()
			f2.Cancel()

			r, _ := f3.Get()
			convey.So(r, convey.ShouldBeNil)
		})
	})
}

func TestWrap(t *testing.T) {
	convey.Convey("Test Wrap a value", t, func() {
		r, err := Wrap(10).Get()
		convey.So(r, convey.ShouldEqual, 10)
		convey.So(err, convey.ShouldBeNil)
	})
}

func shouldSlicesReSame(actual interface{}, expected ...interface{}) string {
	actualSlice, expectedSlice := reflect.ValueOf(actual), reflect.ValueOf(expected[0])
	if actualSlice.Kind() != expectedSlice.Kind() {
		return fmt.Sprintf("Expected1: '%v'\nActual:   '%v'\n", expected[0], actual)
	}

	if actualSlice.Kind() != reflect.Slice {
		return fmt.Sprintf("Expected2: '%v'\nActual:   '%v'\n", expected[0], actual)
	}

	if actualSlice.Len() != expectedSlice.Len() {
		return fmt.Sprintf("Expected3: '%v'\nActual:   '%v'\n", expected[0], actual)
	}

	for i := 0; i < actualSlice.Len(); i++ {
		if !reflect.DeepEqual(actualSlice.Index(i).Interface(), expectedSlice.Index(i).Interface()) {
			return fmt.Sprintf("Expected4: '%v'\nActual:   '%v'\n", expected[0], actual)
		}
	}
	return ""
}
