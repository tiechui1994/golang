package synch

// A WaitGroup waits for a collection of goroutines to finish.
// The main goroutine calls Add to set the number of
// goroutines to wait for. Then each of the goroutines
// runs and calls Done when finished. At the same time,
// Wait can be used to block until all goroutines have finished.
//
// A WaitGroup must not be copied after first use.

/*
WaitGroup解析:
	WaitGroup就是等待一个集合当中的
*/
func waitGroup() {

}
