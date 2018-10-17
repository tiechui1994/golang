package goroutines

import (
	"sync/atomic"
	)

/*
并发控制的模式:
	方式一: 使用channel
	方式二: 使用WaitGroup
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

