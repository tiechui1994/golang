package concurrent

import (
	"sync/atomic"
)

/*
并发控制的模式:

方式一: 使用channel.

	done := make(chan bool)
	defer close(done)
	for {
		// 并发执行
		go func(word string) {
			time.Sleep(1 * time.Second)
			fmt.Println(word)
			done <- true
		}(word)
	}

	<-done // 阻塞执行


方式二: 使用WaitGroup.
场景: 程序中需要并发, 需要创建多个goroutine, 并且一定要等这些并发全部完成后才继续接下来的程序执行,
WaitGroup的特点是Wait()可以用来阻塞直到队列中的所有任务都完成时才解除阻塞, 而不需要sleep一个固定
的时间来等待.但是其缺点是无法指定固定的goroutine数目.

	var wg sync.WaitGroup
	for {
		wg.Add(1)
		go func(word string) {
			time.Sleep(1 * time.Second)
			defer wg.Done()
			fmt.Println(word)
		}(word)
	}
	wg.Wait() // 阻塞
*/

type ChannelGroup struct {
	channel chan bool
	counter int32
}

func NewChannelGroup() *ChannelGroup {
	return &ChannelGroup{
		channel: make(chan bool),
		counter: 0,
	}
}

func (c *ChannelGroup) Add(delta int32) {
	atomic.AddInt32(&c.counter, delta)
	if c.counter <= 0 {
		close(c.channel)
	}
}

func (c *ChannelGroup) Done() {
	c.Add(-1)
}

func (c *ChannelGroup) Wait() {
	<-c.channel
}
